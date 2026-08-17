# Nested container demo

The nested demo packages the Linux deployment experience into one privileged
Docker-in-Docker parent. It is for demonstrations only. It is not the production
deployment model.

The parent performs these steps:

1. Starts a nested Docker daemon.
2. Builds the Archivist child image from the mounted repository.
3. Downloads the Ollama image and configured models when they are missing.
4. Starts Archivist and Ollama as child containers on an internal network.
5. Exposes nested Archivist through Nginx on parent port `8080`.
6. Blocks new outbound connections after setup.

## Start

The first run requires internet access and can take several minutes:

```sh
docker compose -f deploy/demo-parent/compose.yml up --build -d
docker compose -f deploy/demo-parent/compose.yml logs -f
```

Wait until the log prints `Demo ready on port 8080` before running verification
or opening the application.

The nested Docker state and models persist in the
`demo-parent_demo-docker-data` volume. Later starts reuse that cache.

Open `http://HOST_LAN_IP:8080` from the demo machine or another device on its
LAN. The host firewall must permit inbound TCP port 8080.

## Verify

```sh
docker exec archivist-demo-parent verify.sh
```

## Stop

```sh
docker compose -f deploy/demo-parent/compose.yml down
```

To also discard all nested application data, models, and build cache:

```sh
docker compose -f deploy/demo-parent/compose.yml down -v
```

The `-v` operation is destructive.

## Security boundary

The parent requires `privileged: true` to run a nested Docker daemon. Do not use
this design on a shared or production host, and do not mount sensitive host
directories into it. The parent receives only a read-only project mount and its
dedicated Docker-data volume.

For production, use the non-nested deployment in
[`linux-deployment.md`](linux-deployment.md).
