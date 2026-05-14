// Copyright 2024 The Forge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package banner prints the Forge ASCII-art startup banner.
// Color is suppressed when NO_COLOR is set or TERM=dumb.
package banner

import (
	"fmt"
	"io"
	"os"
)

const (
	clrOrange = "\033[38;5;208m"
	clrWhite  = "\033[1;37m"
	clrDim    = "\033[2;37m"
	clrReset  = "\033[0m"
)

func isColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

// Print writes the Forge ASCII-art banner to w.
func Print(w io.Writer) {
	if isColorEnabled() {
		printColor(w)
	} else {
		printPlain(w)
	}
}

// printColor renders the banner with ANSI colors:
//   - orange  → the glowing F inside the anvil, sparks, and tagline accent
//   - white   → the FORGE block letters
//   - dim     → the anvil body / separators
func printColor(w io.Writer) {
	o := clrOrange
	wh := clrWhite
	d := clrDim
	r := clrReset

	// Spark line
	fmt.Fprintf(w, "\n  %s✦  ·  ✦  ·    ✦  ·  ✦  ·    ✦  ·  ✦  ·  ✦%s\n", o, r)
	// Anvil — the F (top bar + vertical bar + middle bar) is orange
	fmt.Fprintf(w, "  %s       _______________________________\n", d)
	fmt.Fprintf(w, "      /      %s██████%s                   |▶\n", o, d)
	fmt.Fprintf(w, "     /        %s██%s                          \\\n", o, d)
	fmt.Fprintf(w, "    (    ◁    %s█████%s            ▷           )\n", o, d)
	fmt.Fprintf(w, "     \\         %s██%s                          /\n", o, d)
	fmt.Fprintf(w, "      \\__________%s██%s______________________/\n", o, d)
	fmt.Fprintf(w, "       |__________%s██%s_____________________|%s\n", o, d, r)
	// Stem and base
	fmt.Fprintf(w, "  %s                  %s██%s\n", d, o, d)
	fmt.Fprintf(w, "             ______|______\n")
	fmt.Fprintf(w, "            |_____________|%s\n", r)
	// FORGE block lettering
	fmt.Fprintf(w, "\n  %s███████╗ ██████╗ ██████╗  ██████╗ ███████╗%s\n", wh, r)
	fmt.Fprintf(w, "  %s██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝%s\n", wh, r)
	fmt.Fprintf(w, "  %s█████╗  ██║   ██║██████╔╝██║  ███╗█████╗  %s\n", wh, r)
	fmt.Fprintf(w, "  %s██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝  %s\n", wh, r)
	fmt.Fprintf(w, "  %s██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗%s\n", wh, r)
	fmt.Fprintf(w, "  %s╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝%s\n", wh, r)
	// Tagline
	fmt.Fprintf(w, "\n         %s< native-ai framework />%s\n", o, r)
	fmt.Fprintf(w, "  %s────────────────────────────────────────%s\n", d, r)
	fmt.Fprintf(w, "   VIBE IT AND SHIP IT.  %sBUILT TO LAST.%s\n\n", o, r)
}

// printPlain renders the banner without any ANSI escape codes.
func printPlain(w io.Writer) {
	fmt.Fprint(w, "\n  *  +  *  +    *  +  *  +    *  +  *  +  *\n")
	fmt.Fprint(w, "       _______________________________\n")
	fmt.Fprint(w, "      /      ######                   |>\n")
	fmt.Fprint(w, "     /        ##                          \\\n")
	fmt.Fprint(w, "    (   <     #####            >           )\n")
	fmt.Fprint(w, "     \\         ##                          /\n")
	fmt.Fprint(w, "      \\__________##______________________/\n")
	fmt.Fprint(w, "       |__________##_____________________|  \n")
	fmt.Fprint(w, "                  ##\n")
	fmt.Fprint(w, "             ______|______\n")
	fmt.Fprint(w, "            |_____________|\n")
	fmt.Fprint(w, "\n  ███████╗ ██████╗ ██████╗  ██████╗ ███████╗\n")
	fmt.Fprint(w, "  ██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝\n")
	fmt.Fprint(w, "  █████╗  ██║   ██║██████╔╝██║  ███╗█████╗  \n")
	fmt.Fprint(w, "  ██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝  \n")
	fmt.Fprint(w, "  ██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗\n")
	fmt.Fprint(w, "  ╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝\n")
	fmt.Fprint(w, "\n         < native-ai framework />\n")
	fmt.Fprint(w, "  ----------------------------------------\n")
	fmt.Fprint(w, "   VIBE IT AND SHIP IT.  BUILT TO LAST.\n\n")
}
