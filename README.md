System for managing and monitoring a small water system.

The system consists of a single well and a [small off-the-shelf MQTT
Sparkplug B edge node](https://www.opto22.com/products/groov-rio) for reading 
4-20mA sensors.

Development plans:

1. Monitoring with [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) receiver
   for [MQTT Sparkplug B](https://projects.eclipse.org/projects/iot.tahu) metrics.
   - Receiver runs an MQTT broker using https://github.com/mochi-co/mqtt
   - Receiver acts as the primary "SCADA" Host.
2. [EPAnet](https://www.epa.gov/water-research/epanet) simulation
   - TODO: see [Open Water Analytics](http://wateranalytics.org/)
