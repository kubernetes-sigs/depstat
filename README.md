# Dependency Analyzer POC

Proof of concept for a dependency analyzer CLI tool.

### Overall workflow

The main command is been worked upon in the [`cmd/showdep.go`](./cmd/showdep.go) file. Clones of standard commands are present in `cmd/*clone.go` files.

1. See original output using:

   1. `wget https://github.com/kubernetes/kubernetes/pull/98946.patch`
   2. `grep b/LICENSES 98946.patch | grep -v diff | cut -f 4- -d '/' | sed 's/\/LICENSE//' | xargs -L 1 echo`

2. See output of tool (work in progress): `go run main.go showdep -u https://github.com/kubernetes/kubernetes/pull/98946.patch`

### Clones of standard commands

To check out individual commands:

1. `grep`:
   1. Run `go run main.go grepclone --file ./grepCloneTest.txt -s From` to see the lines having "From" in the "grepCloneTest.txt" file.
2. `wget`:
   1. Run `go run main.go wgetclone -u https://github.com/kubernetes/kubernetes/pull/98946.patch -f output2.txt` to get the output of the "URL" in an "output2.txt" file.

### References

1. https://hackmd.io/@XYdYH0X5SYC3DUYFF5Wylg/rJimEo--u
2. https://www.kaynetik.com/blog/simple-cli-tool/
3. https://github.com/jeremywho/gowget
4. https://stackoverflow.com/questions/39859222/golang-how-to-overcome-scan-buffer-limit-from-bufio
