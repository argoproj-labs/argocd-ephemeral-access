FROM alpine:3.21.3

WORKDIR /workspace

ADD scripts/plugin-installer.sh .

RUN chown -R nobody:nobody /workspace

# The alpine nobody user
USER 65534

ENTRYPOINT ["/workspace/plugin-installer.sh"]
