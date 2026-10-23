# Week 1 Design Note

I used configuration validation to check the node ID, HTTP address and timeout before starting the service. This helps prevent invalid settings from causing problems when the application runs. The node ID identifies the node, while the HTTP address tells clients where to connect.

The health endpoint, `/healthz`, returns JSON containing the node ID and an `ok` status. This allows a client to check whether the node is responding, but it does not prove that another node is reachable. For testing, I ran two separate nodes on loopback addresses with different ports. This allowed me to check that they could run independently without exposing the service to other networks. I also tested invalid input, an unavailable port and a duplicate port. Strict validation was chosen over accepting a wider range of configuration values because it is suitable for this teaching project.
