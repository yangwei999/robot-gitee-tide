FROM openeuler/openeuler:23.03 as BUILDER
RUN dnf update -y && \
    dnf install -y golang && \
    go env -w GOPROXY=https://goproxy.cn,direct

MAINTAINER zengchen1024<chenzeng765@gmail.com>

# build binary
WORKDIR /go/src/github.com/opensourceways/robot-gitee-tide
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 go build -a -o robot-gitee-tide -buildmode=pie --ldflags "-s -linkmode 'external' -extldflags '-Wl,-z,now'" .

# copy binary config and utils
FROM openeuler/openeuler:22.03
RUN dnf -y update && \
    dnf in -y shadow && \
    dnf remove -y gdbserver && \
    groupadd -g 1000 tide && \
    useradd -u 1000 -g tide -s /sbin/nologin -m tide && \
    echo > /etc/issue && echo > /etc/issue.net && echo > /etc/motd && \
    mkdir /home/tide -p && \
    chmod 700 /home/tide && \
    chown tide:tide /home/tide && \
    echo 'set +o history' >> /root/.bashrc && \
    sed -i 's/^PASS_MAX_DAYS.*/PASS_MAX_DAYS   90/' /etc/login.defs && \
    rm -rf /tmp/*

USER tide

WORKDIR /opt/app

COPY --chown=tide --from=BUILDER /go/src/github.com/opensourceways/robot-gitee-tide/robot-gitee-tide /opt/app/robot-gitee-tide

RUN chmod 550 /opt/app/robot-gitee-tide && \
    echo "umask 027" >> /home/tide/.bashrc && \
    echo 'set +o history' >> /home/tide/.bashrc

ENTRYPOINT ["/opt/app/robot-gitee-tide"]
