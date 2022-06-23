FROM busybox

WORKDIR /
COPY passwd /etc/passwd
COPY package-loader /

USER "noroot"

ENTRYPOINT ["/package-loader"]
