package optparse

import "strings"

// ProgramNameOrDefault returns the program name used in the default banner.
func (p *Parser) programName() string {
	if p.ProgramName != "" {
		return p.ProgramName
	}
	return "optparse"
}

// Help renders the full help text (MRI #help / #to_s).
func (p *Parser) Help() string {
	var b strings.Builder
	banner := p.Banner
	if banner == "" {
		banner = "Usage: " + p.programName() + " [options]"
	}
	b.WriteString(banner)
	b.WriteByte('\n')
	for _, it := range p.list {
		if it.isSep {
			b.WriteString(it.sep)
			b.WriteByte('\n')
		} else {
			for _, l := range p.summarizeSwitch(it.sw) {
				b.WriteString(l)
			}
		}
	}
	return b.String()
}

// Summarize returns the per-switch/separator help lines (MRI #summarize), each
// already terminated with "\n". It does not include the banner.
func (p *Parser) Summarize() []string {
	var out []string
	for _, it := range p.list {
		if it.isSep {
			out = append(out, it.sep+"\n")
		} else {
			out = append(out, p.summarizeSwitch(it.sw)...)
		}
	}
	return out
}

func (p *Parser) summarizeSwitch(sw *switchDef) []string {
	indent := p.SummaryIndent
	width := p.SummaryWidth
	left := p.buildLeft(sw)
	desc := append([]string(nil), sw.desc...)
	var lines []string
	if len(left) <= width {
		var first string
		if len(desc) > 0 {
			first = desc[0]
			desc = desc[1:]
		}
		padded := left + strings.Repeat(" ", width-len(left))
		line := indent + padded
		if first != "" {
			line = strings.TrimRight(line+" "+first, " \t\n\r\v\f")
		}
		lines = append(lines, line+"\n")
	} else {
		lines = append(lines, indent+left+"\n")
	}
	for _, d := range desc {
		lines = append(lines, indent+strings.Repeat(" ", width)+" "+d+"\n")
	}
	return lines
}

func (p *Parser) buildLeft(sw *switchDef) string {
	var argstr string
	switch {
	case sw.mandatory:
		if sw.arg != "" {
			argstr = " " + sw.arg
		}
	case sw.optional:
		if sw.arg != "" {
			argstr = " [" + sw.arg + "]"
		}
	}
	long := append([]string(nil), sw.long...)
	if sw.negatable {
		for i, l := range long {
			long[i] = strings.Replace(l, "--", "--[no-]", 1)
		}
	}
	shortStr := strings.Join(sw.short, ", ")
	longStr := strings.Join(long, ", ")
	switch {
	case shortStr != "" && longStr != "":
		return shortStr + ", " + longStr + argstr
	case shortStr != "":
		return shortStr + argstr
	default:
		return "    " + longStr + argstr
	}
}
