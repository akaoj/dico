FROM fedora:32

ARG USER_ID
RUN adduser --create-home --uid ${USER_ID} dev

RUN dnf install -y \
		golang \
		make \
	&& dnf clean all

WORKDIR /repo
USER dev

CMD ["/bin/bash"]
