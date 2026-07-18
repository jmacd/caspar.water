CREATE TABLE wd AS SELECT timestamp, "well_depth_value.avg" AS well_depth_value FROM read_parquet('wd/*.parquet');
CREATE TABLE sp AS SELECT timestamp, "system_pressure_value.avg" AS system_pressure_value FROM read_parquet('sp/*.parquet');
CREATE TABLE source AS SELECT COALESCE(wd.timestamp, sp.timestamp) AS timestamp, wd.well_depth_value, sp.system_pressure_value FROM wd FULL OUTER JOIN sp USING (timestamp);

-- Tunables (recalibrated to real data)
-- drift_tol=0.03 psi/h, scale=0.02, sample interval 60s (reduced 1m)
CREATE TABLE per_day AS
WITH rest AS (
  SELECT timestamp, system_pressure_value AS pressure, CAST(timestamp AS DATE) AS rest_day
  FROM source
  WHERE system_pressure_value IS NOT NULL
    AND EXTRACT(HOUR FROM timestamp) BETWEEN 9 AND 11
    AND well_depth_value IS NOT NULL AND well_depth_value >= 42.0
),
per_hour AS (
  SELECT rest_day, date_trunc('hour', timestamp) AS hour_start, count(*) AS n,
    regr_slope(pressure, EXTRACT(EPOCH FROM timestamp)/3600.0) AS slope
  FROM rest GROUP BY rest_day, date_trunc('hour', timestamp) HAVING count(*) >= 10
),
per_hour_score AS (
  SELECT rest_day, (n*60.0) AS rest_s,
    1.0/(1.0+exp(-((abs(slope)-0.03)/0.02))) AS p_window
  FROM per_hour
)
SELECT rest_day,
  SUM(LEAST(rest_s/3600.0,1.0)*p_window)/NULLIF(SUM(LEAST(rest_s/3600.0,1.0)),0) AS leak_score,
  LEAST(SUM(rest_s)/3600.0,1.0) AS leak_confidence
FROM per_hour_score GROUP BY rest_day;
