# Dependency Analyzer POC

### Overall workflow

1. Implement functionality of `wget` and fetch the GitHub patch file from URL which is entered as an argument.
2. Process this output to a text file.
3. Implement functionality of `grep` to process this file to get the list of dependencies from it.
4. Provide output in a customized format based on the command run or flags used.

### Current Progress

To test all commands first -> Clone the repo. `cd` into the project folder.

1. Implemented a basic version of `grep`
   1. Run `go run main.go showdep --file ./something.txt -s From` to see the lines having "From" in the the "something.txt" file.
2. Implemented a basic version of `wget`
   1. Run `go run main.go wgetclone -u https://github.com/kubernetes/kubernetes/pull/98946.patch -f output.txt` to get the output of the "URL" in an "output.txt" file.

### References

1. https://hackmd.io/@XYdYH0X5SYC3DUYFF5Wylg/rJimEo--u
2. https://www.kaynetik.com/blog/simple-cli-tool/
3. https://github.com/jeremywho/gowget
4. https://stackoverflow.com/questions/39859222/golang-how-to-overcome-scan-buffer-limit-from-bufio
