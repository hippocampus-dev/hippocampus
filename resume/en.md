# Kai Aihara

| Software Engineer | Tokyo, Japan |

## Summary

Joined LIFULL Co., Ltd. as a new graduate in April 2015.

While working as an Individual Contributor in the Platform Engineering and Site Reliability Engineering domains, I also serve as the sole Senior Principal Engineer, responsible for technical strategy planning, company-wide system architecture, and PSIRT.

Launched a Kubernetes-based internal PaaS team in 2018 and have been leading it to this day.

## Work Experience

### LIFULL Co., Ltd.

| Senior Principal Engineer | Tokyo, Japan | Apr. 2015 - Present |

I have consistently worked in the Platform Engineering domain to deliver scalable impact in realizing the corporate message "Make all LIFEs FULL."

Immediately after joining as a new graduate in 2015, I was assigned to a team handling AWS migration accompanying Microservices transformation. I led zero-downtime migrations of numerous web servers and data stores, as well as the architectural renewal of Solr, the full-text search engine that forms the core of the business.

Concerned about the reinvention of the wheel caused by Microservices transformation, I proposed the adoption of a Kubernetes-based PaaS that I had been developing as a hobby, launched the team in 2018, and have been growing it as a platform that extends beyond Kubernetes to this day.

- Launch, development, and technical management of internal PaaS team
    - From the team's launch in 2018 to the present, I have served as Tech Lead, handling not only development but also member training and team building, managing everything except people management such as performance reviews
    - Since 2020, we have been accepting members from the Sapporo development site, establishing a remote work environment as a multi-location team with Tokyo
    - Related outputs
        - [About LIFULL's Company-wide Application Execution Platform KEEL](https://www.docswell.com/s/LIFULL/5QL3JZ-LIFULL%E3%81%AE%E5%85%A8%E7%A4%BE%E3%82%A2%E3%83%95%E3%82%9A%E3%83%AA%E3%82%B1%E3%83%BC%E3%82%B7%E3%83%A7%E3%83%B3%E5%AE%9F%E8%A1%8C%E5%9F%BA%E7%9B%A4%20KEEL%20%E3%81%AB%E3%81%A4%E3%81%84%E3%81%A6)
- Development and operation of KEEL, a Kubernetes-based internal PaaS
    - Built on multi-tenant, single-cluster Kubernetes aimed at cost optimization through resource consolidation, adopted service mesh with Istio since version 0.2.0, and supports serverless workloads with Knative
    - Equipped with an observability platform using Prometheus, Thanos, Grafana Loki, Fluentd, Grafana Tempo, and Pyroscope, with Spinnaker adopted for deployment
    - Related outputs
        - [Introducing Istio to Production (2018/12/16)](https://www.lifull.blog/entry/2018/12/16/235900)
        - [Summary of Kubernetes Ecosystem Supporting LIFULL 2020 Edition (2020/08/03)](https://www.lifull.blog/entry/2020/08/03/113000)
- Development of general-purpose AI agent implementation for product integration and LLMOps
    - At the release timing of gpt-3.5-turbo, I proactively developed a general-purpose AI agent implementing what is now known as an Agent Loop, anticipating future product integration and LLMOps as a platform
    - Simultaneously released an internal general-purpose chatbot using this agent, which, along with deployment to group companies, [achieved 82% employee usage rate and 42,000 hours of work time reduction as of October 2024](https://lifull.com/news/39363/)
    - Widely used by products due to rate limiting, robust error handling, and comprehensive observability
    - Related outputs
        - [Developing an Infinitely Scalable General-purpose AI (Tentative) Without Using OpenAI Assistants API (2023/11/16)](https://www.lifull.blog/entry/2023/11/16/170000)
        - [Running MCP Servers Relatively Safely (2025/05/08)](https://www.lifull.blog/entry/2025/05/08/170000)
- Development of code generator to realize PaaS experience
    - By entering required fields for Custom Resource Definition and executing, automatically generates everything needed for application development cycle including not only Kubernetes Manifests but also operational documentation, various GitHub Actions, monitoring settings, dashboards, and deployment settings including Content Trust for J-SOX compliance
    - As a code generator, it can opt-in to repositories not using Kubernetes, and has been deployed to all actively developed internal repositories to reuse the Content Trust mechanism and various GitHub Actions, maintaining all repositories in a healthy state through self-update functionality
    - Related outputs
        - [Getting a Production Ready Environment on Kubernetes with a Single Command (2021/03/30)](https://www.lifull.blog/entry/2021/03/30/100000)
- Development of boilerplate for developing CloudNative web applications
    - Provides boilerplates for Go, TypeScript, and Python equipped with OpenTelemetry, Graceful Shutdown, structured logging, profiling, and optimal Dockerfiles to support platform users
    - Combined with the aforementioned code generator, using the boilerplate enables setup in 5 minutes
    - Over 90% of new development repositories use these boilerplates
- Provision of fully-managed Kafka/Redis/memcached/Qdrant clusters
    - Provides frequently used data stores from applications as SaaS within the Kubernetes cluster, allowing users to provision these data stores by simply adding a few lines of configuration to the aforementioned code generator
    - Related outputs
        - [Platform Engineering Approach for Promoting LLM Utilization (2023/07/05)](https://www.lifull.blog/entry/2023/07/05/090000)
- Platform feature expansion through Kubernetes Operator, Prometheus Exporter, proxy-wasm, and middleware development
    - In addition to standard features like Kubernetes Operator for realizing so-called Preview Environments, implemented various software including authorization mechanisms with proxy-wasm and Linux kernel layer metrics collection with eBPF, proactively addressing developer needs
    - Expanded platform functionality broadly beyond Kubernetes, including reducing communication costs outside the Kubernetes cluster through TCP Proxy development for route optimization
    - Related outputs
        - [Implementing a Small Route Optimization Middleware to Reduce All Inter-AZ Communication (2024/09/03)](https://www.lifull.blog/entry/2024/09/03/070000)
        - [eBPF Filling the Observability Gaps in Kubernetes Clusters (2023/11/21)](https://www.lifull.blog/entry/2023/11/21/170000)
        - [Automation with GitHub Actions Self-hosted Runners on Kubernetes (2020/06/03)](https://www.lifull.blog/entry/2020/06/03/080000)
- Migration and migration support for all applications except those scheduled for deprecation to internal PaaS
    - Related outputs
        - [How LIFULL Migrated (Almost) All Major Services to Kubernetes (2019/12/16)](https://www.lifull.blog/entry/2019/12/16/000000)
