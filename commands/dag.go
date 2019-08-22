// Package commands implements the command to print the blockchain.
package commands

import (
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var dagCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with IPLD DAG objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"get":         dagGetCmd,
		"clear-cache": dagClearCacheCmd,
	},
}

var dagGetCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get a DAG node by its CID",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "CID of object to get"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := GetPorcelainAPI(env).DAGGetNode(req.Context, req.Arguments[0])
		if err != nil {
			return err
		}

		return re.Emit(out)
	},
}

var dagClearCacheCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Purge the cache used during transferring of piece data",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		err := GetPorcelainAPI(env).ClearTempDatastore(req.Context)
		if err != nil {
			return err
		}

		return re.Emit("Cache cleared")
	},
}
