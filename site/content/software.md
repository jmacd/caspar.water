
	<section id="software" class="content-section">
	  <h2 class="section-title">Software</h2>

	  <h4>Open-source</h4>

	  <p>Caspar Water System thanks the authors of the many pieces
	    of computer software/system that we depend on, including:

	    <ul>
	      <li><a href="https://www.debian.org/">Debian Linux 🐧</a>
	      <li><a href="https://www.beagleboard.org/">Beagleboard</a>
	      <li><a href="https://www.influxdata.com/lp/influxdb-database">InfluxDB</a>
	      <li><a href="https://opentelemetry.io/docs/collector/">OpenTelemetry Collector</a>
	      <li><a href="https://duckdb.org/">DuckDB</a>
	      <li><a href="https://observablehq.com/framework/">Observable Framework</a>.
	      <li>Many Rust and Golang libraries,
	      especially <a href="https://github.com/mochi-mqtt/server">mochi-mqtt/server</a>,
	      and <a href="https://github.com/simonvetter/modbus">simonvetter/modbus</a>,
	      and the <a href="https://github.com/apache/arrow-rs">Rust Apache Arrow libraries</a>.
	    </ul>
	  </p>

	  <p>Our source code is available under an Apache-2 license
	    at <a href="https://github.com/jmacd/caspar.water">jmacd/caspar.water</a>,
	    including:

	    <ul>
	      <li>Custom OpenTelemetry collector build including
		receivers (modbus, current-loop, mqtt/sparkplug,
		bme280, atlas pH), exporters (influxdb, LCD displays),
		etc.
	      <li>Billing program
		(thanks <a href="https://github.com/johnfercher/maroto">johnfercher/maroto</a>
		for the PDF generator library).
	      <li>Terraform definitions for cloud and station computer
		infrastructure (station, gateway, cloud).
	    </ul>
	  </p>
	    
	  <h4>Duckpond</h4>

	  <p><a href="https://github.com/jmacd/duckpond">Duckpond</a>
	    is a "local-first" Rust software system for managing
	    timeseries from a variety of sources (e.g., random CSV
	    files), based on DuckDB and Parquet files.  This manages a
	    file system of timeseries data and exports to Observable
	    Framework.
	  </p>

	  <p>🚧 Duckpond is being used to publish water monitoring
	    data collected by
	    the <a href="https://noyooceancollective.org/">Noyo Harbor
	    Blue Economy</a> project in a volunteer
	    collaboration. Includes a
	    vendor-specific <a href="https://www.hydrovu.com">HydroVu</a>
	    client library.</p>

	  <p>🚧 Duckpond is being used to publish
	      our <a href="./well_depth/index.html"> high-resolution
	      water monitoring data</a>.
	  </p>

	  <h4>Supruglue</h4>

	  <p><a href="https://github.com/jmacd/supruglue">Supruglue</a>
	    is a C++ programming environment for the Beaglebone/Texas
	    Instruments <em>am335x</em> PRU real-time chip aimed at
	    being low-tech.</p>

	  <p>Mmmm. Proof of concept industrial real-time timer switch
	    (that logs to the cloud), pulse counter, UI-1203 ("Sensus
	    protocol") reader. I would prefer to write such a thing in
	    Rust today, and we're not certain that Texas Instruments
	    will continue producing this chip!</p>

	</section>

