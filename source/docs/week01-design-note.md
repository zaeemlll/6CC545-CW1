# Week 1 Design Note

I used configuration validation to check the node ID, HTTP address and timeout before starting the service. This helps prevent invalid settings from causing problems when the application runs. I also kept the HTTP health endpoint simple. It returns JSON containing the node ID and an "ok" status, so a client can check whether a particular node is responding.

For testing, I used two separate nodes on loopback addresses with different ports. This allowed me to check that they could run independently without exposing the service to other networks. I also tested invalid input, an unavailable port and a duplicate port. One alternative would be to accept more addresses and configuration values, but strict validation is safer for this teaching project.
