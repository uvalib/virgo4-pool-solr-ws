# run application

secs="10"

while true; do
	./bin/virgo4-pool-solr-ws

	echo
	echo "*** program exited; restarting in $secs seconds ***"
	echo

	sleep $secs
done

#
# end of file
#
