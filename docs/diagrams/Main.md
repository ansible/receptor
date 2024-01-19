```mermaid
flowchart TD
Main[Receptor Main] --> NodeCfgInit[NodeCfg Init] 
NodeCfgInit --> Netceptor[Netceptor]
NodeCfgInit --> Workceptor[Workceptor]
NodeCfgInit --> Controlsvc[Controlsvc]
Netceptor --> NetceptorDiagram[Netceptor Diagram]
Netceptor 

click Main "https://github.com/ansible/receptor/blob/devel/cmd/receptor-cl/receptor.go#L49"
click Init "https://github.com/ansible/receptor/blob/devel/pkg/types/main.go#L21"
click Netceptor "https://github.com/ansible/receptor/blob/devel/pkg/netceptor/netceptor.go#L385"
click Controlsvc "https://github.com/ansible/receptor/blob/devel/pkg/controlsvc/controlsvc.go#L161"
click Workceptor "https://github.com/ansible/receptor/blob/devel/pkg/workceptor/workceptor.go#L72"

click NetceptorDiagram "https://github.com/ansible/receptor/blob/devel/docs/diagrams/Netceptor/Netceptor.md"

```