;;  Sleep cycles is like __delay_cycles but dynamic.
    .global sleep_cycles
sleep_cycles:
    ; Subtract this SUB, the return JMP, and the two cycles it took to
    ; get here (or so).
    SUB     r14, r14, 4

$1:
    ; Inner loop takes 2 cycles
    SUB     r14, r14, 2   
    QBLT    $1, r14, 0

    ; Return address
    JMP     r3.w2
