# Offline Linux deployment

Use this deployment for a school or business Linux server. During installation,
the server uses the internet to build the image and download two Ollama models.
After installation, Archivist and Ollama run on an internal Docker network
without a default outbound route. Nginx runs on the host and exposes Archivist
only to the configured LAN.

## Guided setup

From the project directory, start the plain-language setup menu:

```sh
make setup
```

It can install all Linux prerequisites, install only Archivist when the
prerequisites already exist, start the portable demo, build without installing,
or verify an existing deployment. It also explains the model and local-network
choices before making changes.

## One-command Ubuntu/Debian setup

On a clean Ubuntu or Debian server, the bootstrap installs Docker Engine, the
Compose plugin, Nginx, and the application:

```sh
sudo LAN_CIDR=192.168.1.0/24 ./deploy/linux/bootstrap-ubuntu.sh
```

Use a server snapshot or other rollback mechanism when demonstrating automated
host provisioning.

## Manual prerequisites

- Ubuntu 22.04/24.04 or another systemd-based Linux distribution
- Docker Engine with the Docker Compose v2 plugin
- Nginx, curl, and sed
- Internet access during installation
- A free host port 8080

Clone or copy the repository to its permanent location before installing. The
systemd unit records that absolute path.

If Docker Engine, Docker Compose v2, and Nginx are already installed, use the
smaller application installer below.

## Application install

Identify the LAN subnet. Common examples are `192.168.1.0/24`,
`10.20.0.0/16`, or `172.16.5.0/24`. Then run:

```sh
sudo LAN_CIDR=192.168.1.0/24 ./deploy/linux/install.sh
```

The installer performs these steps:

1. Builds the application image.
2. Downloads the configured chat and embedding models.
3. Removes the temporary internet-connected model bootstrap container.
4. Installs an Nginx reverse proxy restricted to `LAN_CIDR`.
5. Installs and starts `archivist.service`.
6. Verifies that the runtime Docker network is internal and has no published
   ports or default outbound route.

Model and application data persist in the `archivist_ollama-data` and
`archivist_archivist-data` Docker volumes.

## Demo

Find the server's LAN address:

```sh
hostname -I
```

From any permitted device on the same LAN, open:

```text
http://SERVER_LAN_IP:8080
```

Create the initial administrator, create a workspace, upload course material,
and ask a question. Model inference, embeddings, documents, and chat history
remain on the server.

If another host firewall is active, permit TCP port 8080 from `LAN_CIDR`. The
Nginx configuration independently rejects clients outside that subnet.

The default internal Docker subnet is `172.30.0.0/24`. If that overlaps an
existing route, set `ARCHIVIST_BACKEND_SUBNET`, `ARCHIVIST_APP_IP`, and
`ARCHIVIST_OLLAMA_IP` consistently when running the installer. The selected
runtime values are persisted in `/etc/archivist/archivist.env`.

## Verify offline isolation

Run:

```sh
sudo ./deploy/linux/verify.sh
```

The check fails if the backend network is not internal, either container has a
default route, a container publishes a host port, or Nginx cannot reach
Archivist.

## Operations

```sh
sudo systemctl status archivist
sudo systemctl restart archivist
sudo journalctl -u archivist
sudo docker compose -p archivist -f deploy/linux/compose.yml logs -f
```

To change the allowed LAN, update `/etc/nginx/conf.d/archivist.conf`, run
`sudo nginx -t`, and reload Nginx.

## Upgrade

Internet access is needed when fetching updated source, images, dependencies,
or models. From the updated repository:

```sh
sudo LAN_CIDR=192.168.1.0/24 ./deploy/linux/install.sh
```

The persistent volumes are reused.

## Remove

Stop and disable the service:

```sh
sudo systemctl disable --now archivist
sudo rm /etc/systemd/system/archivist.service
sudo rm -r /etc/archivist
sudo rm /etc/nginx/conf.d/archivist.conf
sudo systemctl daemon-reload
sudo systemctl reload nginx
```

This intentionally leaves Docker volumes intact. Delete them only when the
stored documents, accounts, chat history, and downloaded models are no longer
needed.
