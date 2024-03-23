```mermaid
%%{init: { 'theme':'dark', 'sequence': {'useMaxWidth':false}, 'fontSize': 14 } }%%
sequenceDiagram
    participant Application
    participant Netceptor

    Application->>+Netceptor: New
    Netceptor-->>-Application: netceptor instance n1
    Application->>+Netceptor:context.Background()
    Netceptor-->>-Application:context.Done()
```