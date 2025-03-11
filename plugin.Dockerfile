FROM alpine:3.21.3

ENV USER_ID=65532
ENV GROUP_ID=65532
ENV USER_NAME=nonroot
ENV GROUP_NAME=nonroot

RUN addgroup -g $GROUP_ID $GROUP_NAME && \
    adduser --shell /sbin/nologin --disabled-password \
    --no-create-home --uid $USER_ID --ingroup $GROUP_NAME $USER_NAME

WORKDIR /workspace

ADD scripts/plugin-installer.sh .

RUN chown -R $USER_ID:$GROUP_ID /workspace

# The nonroot user
USER $USER_ID

ENTRYPOINT ["/workspace/plugin-installer.sh"]
