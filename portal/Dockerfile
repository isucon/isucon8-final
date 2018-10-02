FROM perl:5.28

COPY ./ /app
WORKDIR /app
RUN curl -sL --compressed https://git.io/cpm > /usr/local/bin/cpm && chmod +x /usr/local/bin/cpm
RUN cpanm -n Carton
RUN cpm install && carton install
