import type { TransactionData } from "./api/types";

const districts = ["信義區", "大安區", "松山區", "中山區", "中正區", "大同區", "萬華區", "文山區", "南港區", "內湖區", "士林區", "北投區"];
const roads = ["忠孝東路四段", "信義路三段", "南京東路五段", "民生東路三段", "和平東路二段", "內湖路一段"];
const buildingTypes = ["住宅大樓(11層含以上有電梯)", "華廈(10層含以下有電梯)", "公寓(5樓含以下無電梯)", "透天厝", "套房(1房1廳1衛)"];

/** Generates Taipei sales dated within the 12 months before `now`, pricier near Xinyi. */
export function generateMockTransactions(count = 2000, random: () => number = Math.random, now = new Date()): TransactionData[] {
  const transactions: TransactionData[] = [];
  const start = Date.UTC(now.getUTCFullYear() - 1, now.getUTCMonth(), now.getUTCDate());

  for (let i = 0; i < count; i++) {
    const lat = random() * 0.12 + 24.98;
    const lng = random() * 0.15 + 121.45;
    const distance = Math.hypot(lat - 25.03, lng - 121.56);
    const pricePerPing = Math.max(350_000, 1_300_000 - distance * 8_000_000) + random() * 300_000 - 100_000;
    const area = Math.round((random() * 52 + 8) * 100) / 100;
    const buildingPrice = Math.round(pricePerPing * area / 10_000) * 10_000;
    const parkingPrice = random() < 0.35 ? Math.floor(random() * 200 + 150) * 10_000 : 0;
    const rooms = area < 15 ? 1 : area < 25 ? 2 : area < 40 ? 3 : 4;
    const totalFloors = 4 + Math.floor(random() * 20);
    const district = districts[Math.floor(random() * districts.length)];
    const transactedAt = new Date(start + Math.floor(random() * 365) * 86_400_000);
    const completedAt = new Date(transactedAt);
    completedAt.setUTCFullYear(completedAt.getUTCFullYear() - i % 42);

    transactions.push({
      id: `MOCK${String(i).padStart(6, "0")}`,
      cityCode: "a",
      district,
      address: `台北市${district}${roads[Math.floor(random() * roads.length)]}${1 + Math.floor(random() * 300)}號`,
      transactionType: "房地(土地+建物)",
      buildingType: buildingTypes[Math.floor(random() * buildingTypes.length)],
      mainUse: "住家用",
      totalPrice: buildingPrice + parkingPrice,
      parkingPrice,
      buildingPrice,
      buildingAreaPing: area,
      pricePerPing: Math.round(buildingPrice / area),
      rooms,
      halls: 1,
      baths: Math.max(1, rooms - 1),
      floorLabel: `${1 + Math.floor(random() * totalFloors)}層`,
      totalFloors,
      completedAt: i % 9 === 0 ? undefined : completedAt.toISOString(),
      transactedAt: transactedAt.toISOString(),
      season: "mock",
      lat,
      lng,
      geocodePrecision: "address",
    });
  }

  return transactions;
}
