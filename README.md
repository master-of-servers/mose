# MOSE (Master Of SErvers)

MOSE is a post exploitation tool that enables security professionals with little or no experience with configuration management (CM) technologies to leverage them to compromise environments. CM tools, such as [Puppet](https://puppet.com/) and [Chef](https://www.chef.io/), are used to provision systems in a uniform manner based on their function in a network. Upon successfully compromising a CM server, an attacker can use these tools to run commands on any and all systems that are in the CM server’s inventory. However, if the attacker does not have experience with these types of tools, there can be a very time-consuming learning curve. MOSE allows an operator to specify what they want to run without having to get bogged down in the details of how to write code specific to a proprietary CM tool. It also automatically incorporates the desired commands into existing code on the system, removing that burden from the user. MOSE allows the operator to choose which assets they want to target within the scope of the server’s inventory, whether this is a subset of clients or all clients. This is useful for targeting specific assets such as web servers, or choosing to take over all of the systems in the CM server’s inventory.

> Copyright 2019 National Technology & Engineering Solutions of Sandia, LLC (NTESS). Under the terms of Contract DE-NA0003525 with NTESS, the U.S. Government retains certain rights in this software.