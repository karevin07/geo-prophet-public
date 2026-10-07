package addresspoint

// Source describes where a city publishes its door-number coordinates and how its
// district codes (鄉鎮市區代碼) map to the district names used by 實價登錄.
type Source struct {
	CityCode  string // 實價登錄 file prefix
	DatasetID string // data.gov.tw dataset id; the newest CSV distribution is used
	Districts map[string]string
}

// Sources lists supported cities by 實價登錄 file prefix. Adding a city means adding its
// dataset id and district codes here, provided its CSV keeps the shared column order.
// Code formats differ per city (Taoyuan uses 7 digits), so verify them against the data.
var Sources = map[string]Source{
	"a": {
		CityCode:  "a",
		DatasetID: "155472",
		Districts: map[string]string{
			"松山區": "63000010", "信義區": "63000020", "大安區": "63000030", "中山區": "63000040",
			"中正區": "63000050", "大同區": "63000060", "萬華區": "63000070", "文山區": "63000080",
			"南港區": "63000090", "內湖區": "63000100", "士林區": "63000110", "北投區": "63000120",
		},
	},
	"f": {
		CityCode:  "f",
		DatasetID: "168887",
		Districts: map[string]string{
			"板橋區": "65000010", "三重區": "65000020", "中和區": "65000030", "永和區": "65000040",
			"新莊區": "65000050", "新店區": "65000060", "樹林區": "65000070", "鶯歌區": "65000080",
			"三峽區": "65000090", "淡水區": "65000100", "汐止區": "65000110", "瑞芳區": "65000120",
			"土城區": "65000130", "蘆洲區": "65000140", "五股區": "65000150", "泰山區": "65000160",
			"林口區": "65000170", "深坑區": "65000180", "石碇區": "65000190", "坪林區": "65000200",
			"三芝區": "65000210", "石門區": "65000220", "八里區": "65000230", "平溪區": "65000240",
			"雙溪區": "65000250", "貢寮區": "65000260", "金山區": "65000270", "萬里區": "65000280",
			"烏來區": "65000290",
		},
	},
	"h": {
		CityCode:  "h",
		DatasetID: "157689",
		Districts: map[string]string{
			"桃園區": "6800100", "中壢區": "6800200", "大溪區": "6800300", "楊梅區": "6800400",
			"蘆竹區": "6800500", "大園區": "6800600", "龜山區": "6800700", "八德區": "6800800",
			"龍潭區": "6800900", "平鎮區": "6801000", "新屋區": "6801100", "觀音區": "6801200",
			"復興區": "6801300",
		},
	},
}
