# Changing Ginnunggagap to a Internal Operator Platform

Waypoint bản community đã ngừng bảo trì; HashiCorp dồn hết vào HCP Waypoint dạng SaaS, còn mã nguồn cũ thì nằm archive trên GitHub. Peak của IDP là một thứ **không còn tự chạy được nữa**. 

Sự phụ thuộc của dự án lại nằm ngoài tầm kiểm soát của dự án.

Tài liệu HashiCorp viết: HCP Waypoint cho phép platform team định nghĩa "golden pattern" để lập trình viên tự phục vụ, và các template đó **trừu tượng hoá đi sự phức tạp của việc tạo và quản lý hạ tầng - khỏi tay lập trình viên**. Cả một ngành công nghiệp tự định nghĩa mình bằng việc **giấu hạ tầng đi khỏi dev**. Không ai đặt tên cho cái sự đau khổ mà đống phức tạp ấy bị đẩy *sang*. 

Người đó là các Operator: SRE engineer, infra engineer, platform engineer, sysadmin, DevOps engineer... Và người đó không có category, không có ngân sách, không có hội nghị.

*Ai giữ cho người giữ đền?* - nhưng áp lên tầng platform.

---

## Đề xuất

Một sự định vị lại cách mà các operator đang sử dụng các tool vận hành hệ thống. Tất cả đều hướng về người vận hành, đều bám vào hệ thống có sẵn, đều gắn phần *trí nhớ* vào thứ chỉ giỏi phần *làm*. `Operator Experience` vốn là triết lý trung tâm và `IOP` là danh từ cho triết lý đó.

# Related Work — HashiCorp Waypoint and the Internal Developer Platform category

*Draft material. English, to match the rest of the whitepaper set.*

---

## Timeline

| Date | Event |
|---|---|
| 15 Oct 2020 | Waypoint announced as an open-source project. Pitch: a single `waypoint up` command to build, deploy, and release, with plugins for Docker, Kubernetes, Amazon ECS and Lambda, Google Cloud Run, Azure Container Instances, and Nomad. Builds via Cloud Native Buildpacks. |
| Oct 2022 (HashiConf Global) | Public beta of **HCP Waypoint** announced. The tool is repositioned from a deployment abstraction into an **Internal Developer Platform**: platform teams define "golden patterns," developers self-serve. |
| 9 Aug 2023 | Final community-edition release, v0.11.4. |
| Nov 2023 | HashiCorp announces the re-architecture. Active development of legacy Waypoint ends; the source is archived on GitHub. The new architecture targets SaaS delivery exclusively. |
| — | The `terraform-provider-waypoint` is deprecated; it supported Community Edition, superseded by the HCP provider. |
| 2025 (post-IBM acquisition) | HCP Waypoint continues as a commercial SaaS. "Actions" moves to GA, aimed at managing post-deployment **Day 2 and Day 2+** application scenarios following initial provisioning through templates and add-ons. |

Contemporaneous context at HashiCorp: HCP Vault Secrets was decommissioned (end of sale June 2025), the legacy HCP Terraform free plan reached EOL 31 Mar 2026, and HCP Vagrant is scheduled for end of operations 31 Dec 2026.

---

## Three readings, in descending order of confidence

### 1. The category's centre of gravity pulls toward the developer

Waypoint launched as a deployment abstraction — an artifact an operator would plausibly own — and was repositioned into an IDP two years later. HashiCorp's own documentation states the purpose plainly: templates and add-ons "abstract away the complexities of creating and managing compliant infrastructure **from developers**."

The definition names who is being relieved of complexity. It does not name who receives it. That asymmetry is the gap this work addresses.

### 2. The self-hostable form was withdrawn first

The community edition — the only form a self-managed operator could run — was archived. What survives is SaaS.

This is worth stating precisely, because it is a fact about **delivery model**, not about product failure: HCP Waypoint is alive and shipping. The observation is narrower and sharper. The party who lost access when the model changed was the operator running their own infrastructure, which is precisely the constituency an Internal Operator Platform would serve. A platform for people who must own their infrastructure cannot itself be un-ownable.

### 3. Day 2 arrives as an addendum

HCP Waypoint's "actions" feature exists to handle post-deployment Day 2 and Day 2+ scenarios *after* provisioning through templates. Day 2 is bolted onto a Day 1 abstraction.

This inverts the position taken here, where Day 2 is the premise rather than the extension.

---

## Honest counter-argument (include this; do not omit it)

Waypoint's open-source discontinuation admits explanations that have nothing to do with the operator/developer asymmetry:

- **Late to market.** Backstage and its commercial descendants occupied the IDP category first. A practitioner review after HashiDays 2025 judged the tool excellent but arriving too late against competitors, and hard to picture in a mature enterprise at scale.
- **Positioning confusion.** The same review notes that colleagues and clients frequently did not understand what the tool was — unsurprising for a product whose category changed mid-life.
- **Commercial strategy.** The SaaS pivot fits a company-wide pattern rather than a Waypoint-specific judgment.

Attributing the outcome primarily to the IDP category's blindness toward operators would be motivated reasoning. The defensible claim is narrower: **Waypoint's history illustrates the asymmetry; it does not prove it caused anything.**

---

## Category boundaries

Three categories are routinely conflated and should be separated before the argument is made:

| Category | Examples | What it is |
|---|---|---|
| **Forge** | GitLab, GitHub | A place where the software lifecycle happens. Source, CI, registry, issues. Not an abstraction over infrastructure. |
| **IDP** | Backstage, Port, Humanitec, HCP Waypoint | An abstraction layer placed *between* developers and infrastructure, to hide the latter. |
| **IaC / control plane** | Terraform, Crossplane, Pulumi | A declarative interface for provisioning the infrastructure itself. |

GitLab is the strongest available evidence for the thesis rather than a counter-example. It offers an extensive feature surface for developers, while the operator responsible for keeping a self-managed instance alive — runner fleet, container registry, object storage, Geo replication, upgrade paths — is served by documentation rather than by tooling. The forge is a control plane, and it has no control plane of its own.

---

## References

- https://github.com/hashicorp/waypoint
- https://www.infralovers.com/blog/2023-11-09-the-new-hashicorp-waypoint/
- https://developer.hashicorp.com/hcp/docs/waypoint
- https://developer.hashicorp.com/waypoint/tutorials/hcp-waypoint/get-started-hcp
- https://www.infoq.com/news/2022/10/waypoint-beta-hcp
- https://github.com/hashicorp/terraform-provider-waypoint
- https://www.devoteam.com/expert-view/hashidays-2025-hashicorp-maintains-course-after-ibm-acquisition/
- https://thenewstack.io/qa-hashicorp-cto-armon-dadgar-on-idps-and-iac/
