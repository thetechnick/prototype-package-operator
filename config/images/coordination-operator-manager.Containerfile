FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY coordination-operator-manager /

USER "noroot"

ENTRYPOINT ["/coordination-operator-manager"]
