# goFSL

- End-to-end encrypted file hosting using **AES-GCM**
- File metadata encryption
- Supports large files (does not load files into memory)
- Optional authentication for file upload with multiple users
- Optional file expiry and download limit

| Upload Page                 | Download page               |
|-----------------------------|-----------------------------|
| ![a](docs/screenshot-1.png) | ![b](docs/screenshot-2.png) |

## Quickstart with docker
```shell
# Create the data directory
mkdir -p data
# Create an empty config.toml file, see the example to populate (optional)
touch config.toml

docker run -d \
  --name goFSL \
  --restart always \
  -p 8080:8080 \
  -v "$(pwd)/data:/data" \
  -v "$(pwd)/config.toml:/config.toml" \
  ghcr.io/kioubit/gofsl:latest
```
