package addresspoint

import "math"

// TWD97 TM2 (EPSG:3826) on the GRS80 ellipsoid: central meridian 121°E, scale 0.9999, false easting 250 km.
const (
	grs80SemiMajor  = 6378137.0
	grs80Flattening = 1 / 298.257222101
	tm2ScaleFactor  = 0.9999
	tm2FalseEasting = 250000.0
	tm2CentralLng   = 121.0
)

// TWD97ToWGS84 converts TWD97 TM2 metres to latitude and longitude in degrees.
// TWD97 and WGS84 differ by less than a metre, which is below address point accuracy.
func TWD97ToWGS84(x, y float64) (lat, lng float64) {
	e2 := grs80Flattening * (2 - grs80Flattening)
	ePrime2 := e2 / (1 - e2)
	e1 := (1 - math.Sqrt(1-e2)) / (1 + math.Sqrt(1-e2))

	x -= tm2FalseEasting
	mu := y / tm2ScaleFactor / (grs80SemiMajor * (1 - e2/4 - 3*e2*e2/64 - 5*e2*e2*e2/256))
	footprint := mu +
		(3*e1/2-27*math.Pow(e1, 3)/32)*math.Sin(2*mu) +
		(21*e1*e1/16-55*math.Pow(e1, 4)/32)*math.Sin(4*mu) +
		(151*math.Pow(e1, 3)/96)*math.Sin(6*mu) +
		(1097*math.Pow(e1, 4)/512)*math.Sin(8*mu)

	sin, cos, tan := math.Sin(footprint), math.Cos(footprint), math.Tan(footprint)
	c := ePrime2 * cos * cos
	t := tan * tan
	n := grs80SemiMajor / math.Sqrt(1-e2*sin*sin)
	r := grs80SemiMajor * (1 - e2) / math.Pow(1-e2*sin*sin, 1.5)
	d := x / (n * tm2ScaleFactor)

	latRadians := footprint - (n*tan/r)*(d*d/2-
		(5+3*t+10*c-4*c*c-9*ePrime2)*math.Pow(d, 4)/24+
		(61+90*t+298*c+45*t*t-252*ePrime2-3*c*c)*math.Pow(d, 6)/720)
	lngRadians := (d - (1+2*t+c)*math.Pow(d, 3)/6 +
		(5-2*c+28*t-3*c*c+8*ePrime2+24*t*t)*math.Pow(d, 5)/120) / cos

	return latRadians * 180 / math.Pi, tm2CentralLng + lngRadians*180/math.Pi
}
