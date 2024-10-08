FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY ephemeral-access .
USER 65532:65532
ENTRYPOINT ["/ephemeral-access"]
