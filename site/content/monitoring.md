        <section id="monitoring" class="content-section">
          <h2 class="section-title">Monitoring</h2>

	  <p>Owner/operator Joshua MacDonald is a software engineer
	    with professional experience in telemetry systems, hence
	    our monitoring system uses "cloud-native" software
	    practices. We monitor five instruments:

	    <ul>
	      <li><strong>Well depth:</strong> measures the height of
		the water column relative to the bottom of the well.
	      <li><strong>Chlorine tank level:</strong> lets us
		observe that the chlorine pump is operational.
	      <li><strong>Water tank level:</strong> tells us how much
		treated water is in storage.
	      <li><strong>System pressure:</strong> lets us observe
		dynamic pressure and see that the aeration pump is running.
	      <li><strong>pH level:</strong> An in-tank probe measures
		the pH of the water, lets us see that our aeration
		process is effective.
	    </ul>
	  </p>

          <p>Operators access
            our <a href="https://casparwater.us:8086">Influxdb</a>
            instance with live monitoring data collected through
            several OpenTelemetry Collectors.</p>

	  <p>We have high-resolution well depth measurements dating
	    back to August 2022, with which we can see the history of
	    leaks, leak repairs, faucets left running, and other kinds
	    of fine detail about our impact on the aquifer.</p>

	  <p>🚧 <a href="well_depth/index.html">Our well-depth data is now online.
	    We are working to have it update automatically</a>. </p>
            
        </section>
