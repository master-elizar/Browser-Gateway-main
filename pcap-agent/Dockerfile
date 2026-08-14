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
# Runs as root inside the container rather than dropping to a non-root user via setcap:
# Alpine's setcap/getcap live in a separate package (name/availability isn't reliably
# checkable from a dev sandbox with no package-registry access) whose absence would break
# this build outright, and the extra hardening it buys is marginal for a container this
# narrowly scoped already -- no network listener, no user input, a single static command,
# every other capability dropped, no-new-privileges, read-only rootfs, isolated to exactly
# one session's netns for exactly that session's lifetime. CapAdd: NET_RAW at the container
# level (see orchestrator.CreatePcapSidecar) is what actually grants the capability; this
# just needs to not fight it by dropping to a user Docker didn't apply it to.
FROM alpine:3.20

RUN apk add --no-cache tcpdump

ENTRYPOINT ["tcpdump"]
