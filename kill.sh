p3=""
p2=""
p1=""
p0=""
for v in $(ps)
do
    p3=$p2
    p2=$p1
    p1=$p0
    p0=$v
    if [[ $p0 =~ /var/folders/tl/ ]]
    then
        kill $p3
    fi
done
