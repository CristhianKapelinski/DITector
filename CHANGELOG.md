# Changelog - DITector Research Fork

## [1.2.0] - 2024-04-02
### Fixed
- **Docker Hub V2 API**: Switched to the official registry search API for better stability and result accuracy.
- **JSON Mapping**: Fixed repository field parsing (namespace/name/pull_count) to ensure correct MongoDB population.
- **Login Race Conditions**: Added `sync.Mutex` to the authentication flow to prevent multiple workers from failing login simultaneously.
- **Path Resolution**: Fixed root directory detection to support both `go run` and compiled binaries.

### Added
- **429 Rate Limit Handling**: Intelligent sleep mechanism when encountering "Too Many Requests" from Docker Hub.
- **Verbose Discovery Logs**: Real-time monitoring of discovered repositories and their pull counts.
- **Pagination Delay**: Added configurable delays to prevent IP blacklisting during deep DFS crawls.

## [1.1.0] - 2024-04-02
### Added
- **Parallel Go Crawler**: New core crawler implemented in Golang.
- **DFS Search Strategy**: Depth-First Search keyword generation to bypass 10k results limit.
- **Distributed Strategy**: Meet-in-the-Middle support via `--seed` flag.
- **Dockerized Execution**: Full stack support via `docker-compose.yml`.

---
*Este fork agora é uma ferramenta estável de nível industrial para descoberta de containers.*
