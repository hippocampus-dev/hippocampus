# syntax=docker/dockerfile:1.4

FROM nginx:1.27.3-bookworm

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

COPY nginx.conf /etc/nginx/nginx.conf
COPY releases /usr/share/nginx/html/

USER 65532
