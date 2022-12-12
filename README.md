# Cli utility for download part of git tree by tfs api.

## Install.

```
❯ git clone https://github.com/alekseybb197/tfsgit.git tfsgit
❯ cd tfsgit
❯ make
❯ sudo mv dist/tfsgit /usr/local/bin
❯ sudo chmod +x /usr/local/bin/tfsgit
```

## Configuration.

Cli utility get parameters from environment, command flags or configuration file
.tfsgitrc.yaml in current directory.

```
❯ dist/tfsgit -h
Usage of ./tfsgit:
  -b string
    	branch name (short) (default "master")
  -c string
    	user name and access token (short)
  -d int
    	directory depth (short) (default 10)
  -p string
    	git path (short)
  -q	quiet mode (short)
  -r string
    	repository url (short)
  -t int
    	timeout secs (short) (default 5)
  -tfsbranch string
    	branch name (default "master")
  -tfscred string
    	user name and access token
  -tfsdepth int
    	directory depth (default 10)
  -tfspath string
    	git path
  -tfsquiet
    	quiet mode
  -tfsrepo string
    	repository url
  -tfstimeout int
    	timeout secs (default 5)
```

## Usage example.

```
❯ export tfscred=user:token
❯ export tfsrepo=https://tfs.service/tfs/DefaultCollection/_apis/git/repositories/organization
❯ tfsgit -p=project/source/subdir
```