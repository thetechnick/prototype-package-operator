FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY package-phase-manager /

USER "noroot"

ENTRYPOINT ["/package-phase-manager"]
