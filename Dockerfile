# DOOM over VNC
#
# VERSION               0.1
# DOCKER-VERSION        0.2
# REMOTE-HOST-VERSION   0.3

FROM    ubuntu:14.04
# make sure the package repository is up to date
RUN     apt-get update

# Install dependencies
RUN     apt-get install -y build-essential libsdl-mixer1.2-dev libsdl-net1.2-dev git gcc x11vnc xvfb wget openssh-server && rm -rf /var/lib/apt/lists/*
RUN     mkdir ~/.vnc

# Setup SSH
RUN mkdir /var/run/sshd
RUN echo 'root:VMware123!' | chpasswd
RUN sed -i 's/PermitRootLogin without-password/PermitRootLogin yes/' /etc/ssh/sshd_config

# SSH login fix. Otherwise user is kicked off after login
RUN sed 's@session\s*required\s*pam_loginuid.so@session optional pam_loginuid.so@g' -i /etc/pam.d/sshd

ENV NOTVISIBLE "in users profile"
RUN echo "export VISIBLE=now" >> /etc/profile

EXPOSE 22
CMD ["/usr/sbin/sshd", "-D"]

# Setup a VNC password
RUN     x11vnc -storepasswd 1234 ~/.vnc/passwd

# Setup Doom
RUN     git clone https://github.com/GideonRed/dockerdoom.git
RUN     wget http://distro.ibiblio.org/pub/linux/distributions/slitaz/sources/packages/d/doom1.wad
RUN     cd /dockerdoom/trunk && ./configure && make && make install

# Autostart Doom (might not be the best way to do it, but it does the trick)
RUN     bash -c 'echo "/usr/local/games/psdoom -warp E1M1" >> /root/.bashrc'
