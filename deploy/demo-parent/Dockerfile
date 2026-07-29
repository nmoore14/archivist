FROM docker:28-dind

RUN apk add --no-cache \
    bash \
    curl \
    docker-cli-compose \
    ip6tables \
    iptables \
    nginx

COPY nginx.conf /etc/nginx/http.d/default.conf
COPY entrypoint.sh verify.sh /usr/local/bin/

RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/verify.sh

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
