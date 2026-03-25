FROM alpine:3.19
RUN apk add --no-cache tor && \
    mkdir -p /var/lib/tor/hidden_service && \
    chown -R tor:tor /var/lib/tor/hidden_service && \
    chmod 700 /var/lib/tor/hidden_service
USER tor
ENTRYPOINT ["tor"]
CMD ["-f", "/etc/tor/torrc"]
