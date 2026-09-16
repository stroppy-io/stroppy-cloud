ARG PG_MAJOR=17
FROM docker.stroppy.io/library/postgres:${PG_MAJOR}
ARG PATRONI_VERSION=4.1.5
RUN apt-get update && apt-get install -y --no-install-recommends python3-venv curl \
    && python3 -m venv /opt/patroni \
    && /opt/patroni/bin/pip install --no-cache-dir "patroni[etcd3]==${PATRONI_VERSION}" "psycopg2-binary==2.9.13" \
    && rm -rf /var/lib/apt/lists/*
ENV PATH="/opt/patroni/bin:${PATH}"
COPY entrypoint.sh /usr/local/bin/patroni-entrypoint
RUN chmod 755 /usr/local/bin/patroni-entrypoint
ENTRYPOINT ["patroni-entrypoint"]
CMD ["/etc/stroppy/patroni.yml"]
