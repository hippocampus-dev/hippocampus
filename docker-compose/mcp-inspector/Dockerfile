FROM node:22-bookworm-slim

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

USER 65532

ENTRYPOINT ["npx", "@modelcontextprotocol/inspector@0.12.0", "node", "build/index.js"]
