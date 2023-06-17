// MIT License
//
// Copyright (C) Joshua MacDonald
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

#include <pru_rpmsg.h>
#include <pru_virtio_ids.h>
#include <rsc_types.h>
#include <string.h>

#include <am335x/pru_cfg.h>
#include <am335x/pru_ctrl.h>
#include <am335x/pru_intc.h>

volatile register uint32_t __R30; // output register for PRU
volatile register uint32_t __R31; // input/interrupt register for PRU

struct pru_rpmsg_transport rpmsg_transport;
char rpmsg_payload[RPMSG_BUF_SIZE];
uint16_t rpmsg_src, rpmsg_dst, rpmsg_len;

// Set in resourceTable.rpmsg_vdev.status when the kernel is ready.
#define VIRTIO_CONFIG_S_DRIVER_OK ((uint32_t)1 << 2)

// Sizes of the virtqueues (expressed in number of buffers supported,
// and must be power of 2)
#define PRU_RPMSG_VQ0_SIZE 16
#define PRU_RPMSG_VQ1_SIZE 16

// The feature bitmap for virtio rpmsg
#define VIRTIO_RPMSG_F_NS 0 // name service notifications

// This firmware supports name service notifications as one of its features.
#define RPMSG_PRU_C0_FEATURES (1 << VIRTIO_RPMSG_F_NS)

// sysevt 16 == pr1_pru_mst_intr[0]_intr_req
#define SYSEVT_PRU_TO_ARM 16

// sysevt 17 == pr1_pru_mst_intr[1]_intr_req
#define SYSEVT_ARM_TO_PRU 17

// Chanel 2 is the first (of 8) PRU interrupt output channels.
#define HOST_INTERRUPT_CHANNEL_PRU_TO_ARM 2

// Channel 0 is the first (of 2) PRU interrupt input channels.
#define HOST_INTERRUPT_CHANNEL_ARM_TO_PRU 0

// Interrupt inputs set bits 30 and 31 in register R31.
#define PRU_R31_INTERRUPT_FROM_ARM ((uint32_t)1 << 30) // Fixed, equals channel 0

// (From the internet!)
#define offsetof(st, m) ((uint32_t) & (((st *)0)->m))

// Definition for unused interrupts
#define HOST_UNUSED 255

// HI and LO are abbreviations used below.
#define HI 1
#define LO 0

#pragma DATA_SECTION(my_irq_rsc, ".pru_irq_map")
#pragma RETAIN(my_irq_rsc)

/*
 * .pru_irq_map is used by the RemoteProc driver during initialization. However,
 * the map is NOT used by the PRU firmware. That means DATA_SECTION and RETAIN
 * are required to prevent the PRU compiler from optimizing out .pru_irq_map.
 */
// Note: this is only the interrupts going to the PRU, not the ARM.
struct pru_irq_rsc my_irq_rsc = {
    0, /* type = 0 */
    1, /* number of system events being mapped */
    {
        {SYSEVT_ARM_TO_PRU, HOST_INTERRUPT_CHANNEL_ARM_TO_PRU, 0}, /* {sysevt, channel, host interrupt} */
    },
};

// my_resource_table describes the custom hardware settings used by
// this program.
struct my_resource_table {
  struct resource_table base;

  uint32_t offset[1]; // Should match 'num' in actual definition

  struct fw_rsc_vdev rpmsg_vdev;         // Resource 1
  struct fw_rsc_vdev_vring rpmsg_vring0; // (cont)
  struct fw_rsc_vdev_vring rpmsg_vring1; // (cont)
};

#pragma DATA_SECTION(resourceTable, ".resource_table")
#pragma RETAIN(resourceTable)
// my_resource_table is (as I understand it) how the Linux kernel
// knows what it needs to start the firmware.
struct my_resource_table resourceTable = {
    // resource_table base
    {
        1,    // Resource table version: only version 1 is supported
        1,    // Number of entries in the table (equals length of offset field).
        0, 0, // Reserved zero fields
    },
    // Entry offsets
    {
        offsetof(struct my_resource_table, rpmsg_vdev),
    },
    // RPMsg virtual device
    {
        (uint32_t)TYPE_VDEV,             // type
        (uint32_t)VIRTIO_ID_RPMSG,       // id
        (uint32_t)0,                     // notifyid
        (uint32_t)RPMSG_PRU_C0_FEATURES, // dfeatures
        (uint32_t)0,                     // gfeatures
        (uint32_t)0,                     // config_len
        (uint8_t)0,                      // status
        (uint8_t)2,                      // num_of_vrings, only two is supported
        {(uint8_t)0, (uint8_t)0},        // reserved
    },
    // The two vring structs must be packed after the vdev entry.
    {
        FW_RSC_ADDR_ANY,    // da, will be populated by host, can't pass it in
        16,                 // align (bytes),
        PRU_RPMSG_VQ0_SIZE, // num of descriptors
        0,                  // notifyid, will be populated, can't pass right now
        0                   // reserved
    },
    {
        FW_RSC_ADDR_ANY,    // da, will be populated by host, can't pass it in
        16,                 // align (bytes),
        PRU_RPMSG_VQ1_SIZE, // num of descriptors
        0,                  // notifyid, will be populated, can't pass right now
        0                   // reserved
    },
};

#define WORDSZ sizeof(uint32_t)

// These are word-size offsets from the GPIO register base address.
#define GPIO_CLEARDATAOUT (0x190 / WORDSZ) // For clearing the GPIO registers
#define GPIO_SETDATAOUT (0x194 / WORDSZ)   // For setting the GPIO registers
#define GPIO_DATAOUT (0x13C / WORDSZ)      // For setting the GPIO registers

// Set updates modifies a single bit of a GPIO register.
void set(uint32_t *gpio, int bit, int on) {
  if (on) {
    gpio[GPIO_SETDATAOUT] = 1 << bit;
  } else {
    gpio[GPIO_CLEARDATAOUT] = 1 << bit;
  }
}

// Set up the pointers to each of the GPIO ports
uint32_t *const gpio0 = (uint32_t *)0x44e07000; // GPIO Bank 0  See Table 2.2 of TRM
uint32_t *const gpio1 = (uint32_t *)0x4804c000; // GPIO Bank 1
uint32_t *const gpio2 = (uint32_t *)0x481ac000; // GPIO Bank 2
uint32_t *const gpio3 = (uint32_t *)0x481ae000; // GPIO Bank 3

// uledN toggles the 4 user-programmable LEDs (although the BBB starts
// with them bound to other events, you can echo none >
// /sys/class/leds/$led/trigger to disable triggers and make them
// available for use.
void uled1(int val) { set(gpio1, 21, val); }
void uled2(int val) { set(gpio1, 22, val); }
void uled3(int val) { set(gpio1, 23, val); }
void uled4(int val) { set(gpio1, 24, val); }

// reset_hardware_state enables clears PRU-shared memory, starts the
// cycle counter, clears system events we're going to listen for,
// resets the GPIO bits, etc.
void reset_hardware_state() {
  // Allow OCP master port access by the PRU.
  CT_CFG.SYSCFG_bit.STANDBY_INIT = 0;

  // Clear the system event mapped to the two input interrupts.
  CT_INTC.SICR_bit.STS_CLR_IDX = SYSEVT_ARM_TO_PRU;
  CT_INTC.SICR_bit.STS_CLR_IDX = SYSEVT_PRU_TO_ARM;

  // Reset gpio output.
  const uint32_t allbits = 0x00000000;
  gpio0[GPIO_CLEARDATAOUT] = allbits;
  gpio1[GPIO_CLEARDATAOUT] = allbits;
  gpio2[GPIO_CLEARDATAOUT] = allbits;
  gpio3[GPIO_CLEARDATAOUT] = allbits;
}

// wait_for_virtio_ready waits for Linux drivers to be ready for RPMsg communication.
void wait_for_virtio_ready() {
  volatile uint8_t *status = &resourceTable.rpmsg_vdev.status;

  while (!(*status & VIRTIO_CONFIG_S_DRIVER_OK)) {
    // Wait
  }
}

// setup_transport opens the RPMsg channel to the ARM host.
void setup_transport() {
  // Using the name 'rpmsg-pru' will probe the rpmsg_pru driver found
  // at linux/drivers/rpmsg/rpmsg_pru.c
  char *const channel_name = "rpmsg-pru";
  char *const channel_desc = "Channel 30";
  const int channel_port = 30;

  // Initialize two vrings using system events on dedicated channels.
  pru_rpmsg_init(&rpmsg_transport, &resourceTable.rpmsg_vring0, &resourceTable.rpmsg_vring1, SYSEVT_PRU_TO_ARM,
                 SYSEVT_ARM_TO_PRU);

  // Create the RPMsg channel between the PRU and the ARM.
  while (pru_rpmsg_channel(RPMSG_NS_CREATE, &rpmsg_transport, channel_name, channel_desc, channel_port) !=
         PRU_RPMSG_SUCCESS) {
  }
}

// send_to_arm sends the carveout addresses to the ARM.
void send_to_arm() {
  if (pru_rpmsg_receive(&rpmsg_transport, &rpmsg_src, &rpmsg_dst, rpmsg_payload, &rpmsg_len) != PRU_RPMSG_SUCCESS) {
    return;
  }
  // TODO: No longer the address of the controls struct
  // memcpy(rpmsg_payload, &resourceTable.controls.pa, 4);
  while (pru_rpmsg_send(&rpmsg_transport, rpmsg_dst, rpmsg_src, rpmsg_payload, 4) != PRU_RPMSG_SUCCESS) {
  }
}

uint32_t check_signal() {
  if (__R31 & PRU_R31_INTERRUPT_FROM_ARM) {

    if (CT_INTC.SECR0_bit.ENA_STS_31_0 & (1 << SYSEVT_ARM_TO_PRU)) {
      // This means the control program restarted, needs to know carveout addresses.
      CT_INTC.SICR_bit.STS_CLR_IDX = SYSEVT_ARM_TO_PRU;
      return 1;
    }
  }

  return 0;
}

#define CYCLES_PER_US 200
#define CYCLES_PER_MS (1000 * CYCLES_PER_US)
#define PRE_SETTLE_CYCLES (70 * CYCLES_PER_US)
#define POST_SETTLE_CYCLES (430 * CYCLES_PER_US)
#define HALF_PERIOD_CYCLES (500 * CYCLES_PER_US)
#define DATA_ARRAY_SIZE 256

struct meter_state {
  char data[DATA_ARRAY_SIZE]; // Received data buffer
  int data_index;             // index into received data buffer
  int bitno;                  // bit position we're currently reading
  int parity_check;           // parity check bit
  int done;
};

// Use P9_27 on PRU0 for input via R31[5] (also available on gpio3[19]).
#define INPUT_DATA_R31_MASK (1 << 5)

int read_bit() {
  if (__R31 & INPUT_DATA_R31_MASK) {
    return 1;
  }
  return 0;
}

// Output uses P9_25 on PRU0 for output via R30[7] (also available on gpio3[21]).
#define OUTPUT_DATA_R30_MASK (1 << 7)

void set_clock(int value) {
  if (value) {
    //__R30 |= OUTPUT_DATA_R30_MASK;
    set(gpio3, 21, 1);
    set(gpio1, 17, 0);

  } else {
    set(gpio3, 21, 0);
    set(gpio1, 17, 1);

    //__R30 &= ~OUTPUT_DATA_R30_MASK;
  }
}

#if 0
void next_bit(struct meter_state *meter) {
  int input = read_bit();

  switch (meter->state) {
  case WAIT_FOR_START:
  case READ_BITS:
  case WAIT_FOR_PARITY:
  case WAIT_FOR_STOP:
  default:
    break;
  }
}
#endif

void main(void) {
  // static struct meter_state meter0;

  reset_hardware_state();

  wait_for_virtio_ready();

  setup_transport();

  set_clock(0);

  while (1) {
    __delay_cycles(5000000);
    set_clock(1);
    uled1(1);
    uled2(0);
    uled3(1);
    uled4(0);

    __delay_cycles(5000000);
    set_clock(0);
    uled1(0);
    uled2(1);
    uled3(0);
    uled4(1);
  }
}
