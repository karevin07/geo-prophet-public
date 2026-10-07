# GeoProphet — Data Pipeline & Geospatial Analytics

[繁體中文](README.md) · [English](README.en.md)

A Taiwanese real-estate data engineering and geospatial analytics project. Go ingestion pipelines integrate multiple sources, PostgreSQL/PostGIS stores processed data, and a Gin API and React application expose market insights and data quality.

This is a standalone public showcase of the development project. It does not include the original development history, production configuration, database backups, or downloaded official datasets. Public CSV fixtures and TDX loader tests use synthetic scenarios; their values are not actual market data or investment advice.

## Engineering highlights

| Area | Implementation and reading guide |
| --- | --- |
| Data Pipeline | Multi-source parsing, address normalization, TWD97 → WGS84 conversion, location accuracy, COPY staging and upserts, import records, and reruns: [Import pipeline](docs/DATA_IMPORT.md) |
| Data Quality | Record-count and geocoding-rate checks, import coverage and sample thresholds, an alert queue, and Webhook retries: [Architecture](ARCHITECTURE.md) |
| Backend | Gin delivery/use-case/repository layers, PostGIS GiST indexes, keyset pagination, and streaming CSV exports: [API](docs/API.md) |
| Visualization | MapLibre, Deck.gl, H3, regional comparisons, monthly trends, coverage-gap indicators, React lazy loading, and manual chunks: [User guide](docs/MAP_GUIDE.md) |

## Features and stack

- Map exploration: transaction locations, H3 hexagonal aggregation, filters, and layer settings.
- Transaction feed: filtering and sorting, pagination, detail drawers, map navigation, and API-backed CSV exports.
- Market and regional research: multi-year trends, monthly summaries, grouped analysis, unit-price distributions, and district comparisons.
- Investment research: regional indicator comparisons, a comparison workbench, and liquidity research. Research indicators are not return forecasts.
- Data operations: import records, data-quality checks, and configurable Webhook alerts. Administrative features require additional identity and database configuration.

The frontend uses React, TypeScript, Vite, Tailwind CSS, MapLibre, Deck.gl, H3, and Recharts. The backend uses Go, Gin, and pgx. Storage uses PostgreSQL/PostGIS, supported by SQL migrations, Docker Compose, Vitest, and Go tests.

## Quick start: no database or API keys required

Requirements: Node.js 24+, npm, and Make. The basemap still requires network access; locally generated mock data does not make the map fully offline.

```bash
npm ci
make dev-web
```

Open <http://localhost:3100>. This command explicitly selects frontend mock data and does not call production data services. Rentals, presales, import status, and some investment features require database-backed data and may show empty states in mock mode. CSV export is disabled in frontend-only mock mode.

To try the Go API's mock implementation, run `make dev-api` in another terminal (Go 1.26.8+ required) and check `curl http://127.0.0.1:8080/healthz`. Use `make dev-web-api` to connect the frontend to Gin through `/api`. Do not run it alongside `make dev-web`, as both use the same port.

## Local PostgreSQL/PostGIS environment

Requirements: Docker with the Compose plugin and Make.

```bash
cp .env.example .env
# Edit POSTGRES_PASSWORD first: use a URL-safe password for this local demo.
make docker
```

The Docker project is named `geo-prophet-public`. The web service binds only to `127.0.0.1:3100`; neither the API nor the database publishes a host port. Its data volume is isolated from other projects and does not connect to an existing production database. The initial database is empty: no datasets are downloaded and no background synchronization runs automatically. Stop the stack without deleting its data with `make docker-down`.

Real-data imports require reviewing the source terms and explicitly running the [import commands](docs/DATA_IMPORT.md). An importer CLI is included for research; a production database dump is not.

## Data sources and TDX

The repository retains integrations for Ministry of the Interior real-estate transaction data, TDX, and OSM. Downloaded official datasets and API credentials are not distributed.

- Real-estate transactions: sales, rental, and presale parsers; public CSV fixtures are manually authored synthetic scenarios.
- TDX: an API client and transport-network loader; tests use synthetic station, exit, and route responses, not saved official JSON fixtures.
- OSM: ingestion support is retained. Data and basemaps remain subject to their respective terms and attribution requirements.

Using real TDX data requires supplying your own credentials and explicitly running ingestion. Public Compose does not supply credentials or enable synchronization by default. See [Data sources and licensing](docs/DATA_SOURCES.md) and [Data import](docs/DATA_IMPORT.md).

## Development and verification

```bash
make install
make lint
make test
make build
make help
```

Frontend checks cover TypeScript, Vitest, and Vite builds. Backend checks cover vet, race tests, and server/importer builds. Some Go tests start local HTTP test servers and require loopback sockets. The Go module path retains its existing package identity; it does not imply an external dependency on a private repository.

## Implemented scope and limitations

- Implemented: parsing and normalization, geocoding, batch writes and deduplication, success/failure records, quality alerts, queries, and visualization.
- Current boundary: raw ZIP/CSV files and TDX responses are not durably retained. Analytics primarily use on-demand SQL and use-case aggregation. This is not a complete raw/processed/analytical three-layer platform or lakehouse.
- Future directions: versioned raw sources and checksums, replayable batches, incremental analytical summaries, and reproducible performance benchmarks.

## Documentation and public-release boundaries

[Documentation index](docs/README.md) · [Architecture](ARCHITECTURE.md) · [Data sources](docs/DATA_SOURCES.md) · [Release policy](docs/PUBLIC_RELEASE.md) · [Security](SECURITY.md) · [Contributing](CONTRIBUTING.md)

Supporting documentation is currently primarily in Traditional Chinese. The local demo is not a production deployment template. Administrative access is denied by default, with no demo identity bypass; see [Access boundaries](docs/ACCESS_BOUNDARY.md).

## License

Original project code and documentation are provided under the [MIT License](LICENSE). Third-party dependencies, basemaps, and independently obtained datasets remain subject to their own licenses. This project's MIT license does not relicense government, TDX, or OSM data.
