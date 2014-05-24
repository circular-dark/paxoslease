files=$(find . -name "*.go")
for file in $files;
do
    go fmt $file
done
