# pcap-agent: a minimal, single-purpose packet-capture sidecar.
#
# It exists so the actual browser-engine container never has to hold CAP_NET_RAW --
# per docs/security.md that container stays privileged:false, cap_drop:ALL, non-root, with
# no exception. This image instead runs as its OWN short-lived container, joined to a
# session's browser container via Docker's `network_mode: container:<id>` (sharing its
# network namespace without sharing its filesystem, process list, or capabilities), so only
# this narrow, capability-minimal helper ever gets raw-socket access, scoped to exactly one
# session for exactly the session's lifetime.
FROM alpine:3.20

RUN apk add --no-cache tcpdump libcap && \
    setcap cap_net_raw+eip "$(which tcpdump)" && \
    adduser -D -H -u 10001 pcap

USER pcap
ENTRYPOINT ["tcpdump"]
