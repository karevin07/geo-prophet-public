# GeoProphet

[繁體中文](README.md) · [English](README.en.md)

Explore Taiwan's property transactions alongside address coordinates and rail stations. View sales on a map, follow price trends, and compare districts.

Go downloads and imports the data, PostgreSQL/PostGIS stores and queries it, and React displays maps and charts.

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

## Documentation

[Data import](docs/DATA_IMPORT.md) · [API](docs/API.md) · [User guide](docs/MAP_GUIDE.md) · [Development and database setup](CONTRIBUTING.md) · [More docs](docs/README.md)

## Data and licensing

The repo includes import code and synthetic test data, not downloaded official datasets or API credentials. To use TDX or other sources, configure your own credentials where required and check the [data licenses](docs/DATA_SOURCES.md). The comparison metrics do not predict future price gains.

Code and documentation use the [MIT License](LICENSE). Third-party data and basemaps retain their own licenses.
