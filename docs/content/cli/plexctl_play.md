## plexctl play

Play a media item

```
plexctl play [media_id] [flags]
```

### Options

```
  -h, --help        help for play
      --no-resume   Start playback from the beginning
      --tct         Use the TCT text renderer (legacy terminal video)
      --trailer     Play the primary trailer instead of the full media
```

### Options inherited from parent commands

```
      --config string           config file (default is $HOME/.plexctl.yaml)
      --no-cache                Disable caching
  -o, --output string           Output format (table, json, json-pretty, yaml, csv, txt) (default "table")
      --sort string             column to sort by
      --terminal-video string   Terminal video renderer for Ctrl+P or play (auto, kitty, sixel, tct) (default "auto")
  -v, --verbose count           increase verbosity
```

### SEE ALSO

* [plexctl](plexctl.md)	 - A robust CLI for managing your Plex Media Server

