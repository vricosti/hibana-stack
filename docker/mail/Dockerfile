FROM ubuntu:24.04

LABEL maintainer="Hibana Stack"
LABEL description="Mail server with Postfix, Dovecot, OpenDKIM, and SpamAssassin"

ENV DEBIAN_FRONTEND=noninteractive

# Install mail server packages
RUN apt-get update && apt-get install -y \
    postfix \
    postfix-pgsql \
    dovecot-core \
    dovecot-imapd \
    dovecot-pop3d \
    dovecot-lmtpd \
    dovecot-sieve \
    dovecot-managesieved \
    dovecot-pgsql \
    opendkim \
    opendkim-tools \
    spamassassin \
    spamc \
    rsyslog \
    supervisor \
    ca-certificates \
    openssl \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Create vmail user and group
RUN groupadd -g 5000 vmail && \
    useradd -u 5000 -g vmail -s /usr/sbin/nologin -d /var/mail vmail

# Create required directories
RUN mkdir -p /var/mail/vhosts \
    /var/spool/postfix/private \
    /var/lib/dovecot/sieve \
    /etc/opendkim/keys \
    /run/opendkim \
    /var/log/mail \
    && chown -R vmail:vmail /var/mail/vhosts \
    && chown opendkim:opendkim /run/opendkim /etc/opendkim/keys \
    && chmod 750 /etc/opendkim/keys

# Create default sieve script
RUN echo 'require ["fileinto", "reject", "vacation"];' > /var/lib/dovecot/sieve/default.sieve \
    && sievec /var/lib/dovecot/sieve/default.sieve || true

# Copy supervisor configuration
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose mail ports
# SMTP: 25, Submission: 587, SMTPS: 465
# IMAP: 143, IMAPS: 993
# POP3: 110, POP3S: 995
EXPOSE 25 587 465 143 993 110 995

# Volumes for persistent data
VOLUME ["/var/mail/vhosts", "/etc/opendkim/keys"]

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
