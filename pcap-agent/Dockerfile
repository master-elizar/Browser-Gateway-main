# pcap-agent: a minimal, single-purpose packet-capture sidecar.
#
# It exists so the actual browser-engine container never has to hold CAP_NET_RAW --
# per docs/security.md that container stays privileged:false, cap_drop:ALL, non-root, with
# no exception. This image instead runs as its OWN short-lived container, joined to a
# session's browser container via Docker's `network_mode: container:<id>` (sharing its
# network namespace without sharing its filesystem, process list, or capabilities), so only
# this narrow, capability-minimal helper ever gets raw-socket access, scoped to exactly one
# session for exactly the session's lifetime.
#
# Runs as UID 65532 -- the same distroless "nonroot" UID install.sh already chowns
# $INSTALL_DIR/data to for the backend container -- rather than as root. Linux capabilities
# (unlike setuid-root privilege escalation) apply to a process's capability sets regardless
# of its UID: CapAdd: NET_RAW at the container level (see orchestrator.CreatePcapSidecar)
# grants raw-socket access to this process whether it's root or not, so there's no need for
# root here at all. Matching the data directory's owning UID instead means this container
# gets write access to its one bind-mounted output path (/pcap) through ordinary file
# permissions, with no DAC-bypassing capability needed -- running as root under
# CapDrop: ALL actually has *less* filesystem access than this, since CapDrop: ALL also
# strips CAP_DAC_OVERRIDE, the capability that normally lets root ignore permission bits.
FROM alpine:3.20

RUN apk add --no-cache tcpdump

USER 65532:65532
ENTRYPOINT ["tcpdump"]
