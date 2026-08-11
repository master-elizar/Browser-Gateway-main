FROM debian:bookworm-slim

# ENGINE=chromium | firefox — compose builds two images from the same context.
ARG ENGINE=chromium

ENV DEBIAN_FRONTEND=noninteractive \
    DISPLAY=:99 \
    RESOLUTION=1280x800x24 \
    HOME=/home/browser \
    LANG=C.UTF-8 \
    PIP_BREAK_SYSTEM_PACKAGES=1 \
    BROWSER_ENGINE=${ENGINE}

RUN apt-get update && apt-get install -y --no-install-recommends \
    xvfb \
    x11vnc \
    openbox \
    novnc \
    websockify \
    python3 \
    python3-websocket \
    python3-pip \
    python3-numpy \
    xclip \
    xdotool \
    dnsmasq \
    ffmpeg \
    ca-certificates \
    fonts-liberation \
    fonts-dejavu-core \
    curl \
    procps \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    && if [ "$ENGINE" = "firefox" ]; then \
         apt-get install -y --no-install-recommends firefox-esr; \
       else \
         apt-get install -y --no-install-recommends chromium chromium-sandbox; \
       fi \
    && useradd -m -u 1000 -s /bin/bash browser \
    && mkdir -p /home/browser/data /home/browser/Downloads /home/browser/Uploads /var/log/browser \
    && chown -R browser:browser /home/browser /var/log/browser \
    && pip3 install --no-cache-dir aiortc av mss websockets pillow \
    && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /entrypoint.sh
COPY session_agent.py /session_agent.py
COPY health_agent.py /health_agent.py
COPY dns_tap.py /dns_tap.py
COPY webrtc_agent.py /webrtc_agent.py
COPY control_agent.py /control_agent.py
RUN chmod +x /entrypoint.sh

USER browser
WORKDIR /home/browser

EXPOSE 6080 8090 8091 9222

HEALTHCHECK --interval=3s --timeout=3s --retries=30 \
  CMD python3 -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8090/healthz')"

ENTRYPOINT ["/entrypoint.sh"]
