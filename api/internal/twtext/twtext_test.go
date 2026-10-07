package twtext

import "testing"

func TestNormalize(t *testing.T) {
	if got := Normalize("　臺北市大安區仁愛路四段７９號ＡＢ－１　"); got != "台北市大安區仁愛路四段79號AB-1" {
		t.Fatalf("unexpected normalized text %q", got)
	}
}
