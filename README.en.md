# GeoProphet

[![CI](https://github.com/karevin07/geo-prophet-public/actions/workflows/ci.yml/badge.svg)](https://github.com/karevin07/geo-prophet-public/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![PostgreSQL 16 + PostGIS](https://img.shields.io/badge/PostgreSQL_16-PostGIS-4169E1?logo=postgresql&logoColor=white)
![React 19](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)
![TypeScript](https://img.shields.io/badge/TypeScript-5.8-3178C6?logo=typescript&logoColor=white)
![MapLibre + deck.gl](https://img.shields.io/badge/MapLibre-deck.gl-396CB2?logo=maplibre&logoColor=white)
![Docker Compose](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)

[繁體中文](README.md) · [English](README.en.md)

Explore Taiwan's registered sales, leases, and presale transactions alongside address coordinates, rail stations, and nearby facilities. View sales on a map, follow price trends, compare districts, and estimate the price and rent of a single address.

Go downloads and imports the data, PostgreSQL/PostGIS stores and queries it, and React displays maps and charts.

![Map: H3 hexagons over Taipei coloured by average price per ping, with rail lines and station year-on-year changes](docs/screenshots/map.jpg)

| | |
| --- | --- |
| ![Market analysis: multi-year price trend and monthly sales](docs/screenshots/analysis.jpg) | ![Area overview: a district's prices, rents, presales, and facilities](docs/screenshots/area.jpg) |
| **Market analysis**: filter by building type, price, and size to see prices and sales over several years. | **Area overview**: one district's price rank, rental yield, presale premium, and facilities per km². |
| ![Rents: gross yield by district and rent trend](docs/screenshots/rents.jpg) | ![Estimate: price and rent from nearby sales](docs/screenshots/estimate.jpg) |
| **Rents**: whole-unit rents, gross yields, and trends by district. | **Estimate**: price and monthly rent from comparable sales and leases nearby, with backtested 80% ranges. |
| ![Presale projects: project map, sales, and comparison with nearby resales](docs/screenshots/presales.jpg) | ![Stations: price ranking within 500 m](docs/screenshots/stations.jpg) |
| **Presale projects**: sales pace, floor premiums, and the gap to nearby resale homes. | **Stations**: price rankings around MRT, rail, and high-speed rail stations, and changes after new lines open. |

Screenshots come from a local deployment loaded with the official data. The local trial mode below uses synthetic data, so its numbers differ. The interface is in Traditional Chinese, with an English toggle.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/architecture/architecture-dark.png">
  <img src="docs/architecture/architecture-light.png" alt="GeoProphet: Go ingestion, PostGIS, Gin API, React maps, and quality alerts">
</picture>

[Architecture](ARCHITECTURE.md) · [Open the interactive diagram](docs/architecture/README.md). Diagram labels and most supporting docs are in Traditional Chinese.

## Run locally

Install Node.js 24+, npm, and Make, then run:

```bash
npm ci
make dev-web
```

Open <http://localhost:3100>. This mode uses synthetic data, so no database or API keys are needed. The basemap still needs network access. Rental and presale pages may have no data, and CSV export is disabled.

### Use real data

You need Docker with the Compose plugin. These commands start a local PostGIS database and download a year of Taipei (`CITY=a`) transactions plus facility data:

```bash
cp .env.example .env    # set POSTGRES_PASSWORD to a local password
make docker
make import-city CITY=a SEASON=114S3-115S2
```

Then open <http://localhost:3100>. New Taipei is `f` and Taoyuan is `h`; `CITY=a,f,h` imports all three. `SEASON` takes one season (`115S2`) or a range, and more seasons take longer to download and import. One year, as above (about 5 minutes for Taipei), is enough for the map and market pages; estimates use the last three years of sales, so use `SEASON=112S3-115S2` for them. Rail stations need a TDX key; see [Data import](docs/DATA_IMPORT.md#大眾運輸站點). Check the [data licenses](docs/DATA_SOURCES.md) before downloading.

## Documentation

[Data import](docs/DATA_IMPORT.md) · [API](docs/API.md) · [User guide](docs/MAP_GUIDE.md) · [Development and database setup](CONTRIBUTING.md) · [More docs](docs/README.md)

## Data and licensing

The repo includes import code and synthetic test data, not downloaded official datasets or API credentials. To use TDX or other sources, configure your own credentials where required and check the [data licenses](docs/DATA_SOURCES.md). The comparison metrics do not predict future price gains.

Code and documentation use the [MIT License](LICENSE). Third-party data and basemaps retain their own licenses.
