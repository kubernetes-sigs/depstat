# Dependency Analyzer POC

### Overall workflow

1. Implement functionality of `wget` and fetch the GitHub patch file from URL which is entered as an argument.
2. Process this output to a text file.
3. Implement functionality of `grep` to process this file to get the list of dependencies from it.
4. Provide output in a customized format based on the command run or flags used.

### Current Progress

1. Implemented a basic version of `grep`
   1. Clone the repo. `cd` into the project folder.
   2. Run `go run main.go showdep --file ./something.txt -s From` to see the lines having "From" in the the "something.txt" file.

### References

1. https://hackmd.io/@XYdYH0X5SYC3DUYFF5Wylg/rJimEo--u
2. https://www.kaynetik.com/blog/simple-cli-tool/
