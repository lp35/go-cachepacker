# go-cachepacker

A CLI tool that creates Go module ZIP archives from a local VCS repository, suitable for use with a Go module proxy or cache.

## Why do you need this tool?

This tool is particularly useful when a Go dependency lives in a private repository:

- **No credential leaks**: avoids `url.insteadOf` git rewrite rules that can expose credentials in the global git config.
- **No `replace` directive**: keeps `go.mod` clean and ensures the Go module proxy and checksum database (`go.sum`) are still used for verification.
- **Proxy-compatible output**: the generated ZIP follows the module proxy protocol and produces the same hash as the one a Go proxy would serve, so `go get` can verify the module checksum as usual.

The git repository does not need to be checked out at the target version. The tool resolves the correct commit internally from the version string.

## Usage

```sh
go run . -repo <path-to-repo> -version <version>
```

### Flags

| Flag       | Default      | Description                                                                   |
|------------|--------------|-------------------------------------------------------------------------------|
| `-repo`    | *(required)* | Absolute or relative path to the local git repository                         |
| `-version` | *(required)* | Canonical module version (e.g. `v1.2.3` or a pseudo-version)                  |
| `-out`     | *(auto)*     | Output directory. Defaults to `${GOMODCACHE}/cache/download/<MODULE>/@v`      |

The git ref is derived automatically: for pseudo-versions (e.g. `v0.0.0-20260510095722-d4e5f6a7b8c9`) the embedded commit hash is used. For regular versions the version tag is used directly.

### Example

```sh
go run . -repo /path/to/mymodule -version v1.2.3
```

This produces a `v1.2.3.zip` file in the output directory, containing the module contents as expected by the Go module proxy protocol.

## Build

```sh
go build -o go-cachepacker .
```