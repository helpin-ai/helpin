# Network topology examples

These are illustrative networks, not discovered workspace infrastructure. Keep the `nwdiag` fence when embedding in Helpin Markdown.

## Firewall connecting a DMZ and a private network

Reusing `edge` attaches one firewall to both networks. The two addresses belong to different interfaces on that same node.

```nwdiag
nwdiag {
  network dmz {
    address = "10.10.0.0/24";
    edge [description = "Edge firewall", shape = firewall, address = "10.10.0.1"];
    proxy [description = "Reverse proxy", shape = server, address = "10.10.0.10"];
  }
  network private {
    address = "10.20.0.0/24";
    edge [address = "10.20.0.1"];
    api [description = "Application", shape = server, address = "10.20.0.10"];
    db [description = "Database", shape = database, address = "10.20.0.20"];
  }
}
```

Network membership describes connectivity, not firewall rules or allowed traffic. Add those as grounded annotations outside the diagram.

## Redundant application hosts and a peer link

The group encloses two hosts visually. The peer link describes a separate replication relationship.

```nwdiag
nwdiag {
  network service_net {
    address = "10.30.0.0/24";
    balancer [description = "Load balancer", shape = loadbalancer, address = "10.30.0.5"];
    app_a [description = "App A", shape = server, address = "10.30.0.10"];
    app_b [description = "App B", shape = server, address = "10.30.0.11"];
  }
  group app_pool {
    description = "Application pool";
    app_a;
    app_b;
  }
  app_a -- app_b [label = "Replication", style = dashed];
}
```

## Aligned segments and an explicit path

`row` aligns distinct rails without merging networks. `placement` positions a host relative to its attached rails. These, and `route`, are SimpleDiag extensions; use them when the topology benefits from explicit layout.

```nwdiag
nwdiag {
  network office {
    row = "Access";
    address = "10.40.0.0/24";
    workstation [description = "Workstation", shape = client, address = "10.40.0.10", placement = top];
    gateway [description = "Gateway", shape = router, address = "10.40.0.1"];
  }
  network services {
    row = "Access";
    address = "10.50.0.0/24";
    gateway [address = "10.50.0.1"];
    app [description = "Service", shape = server, address = "10.50.0.10", placement = bottom];
  }
  route workstation -> gateway -> app [label = "Request path"];
}
```
