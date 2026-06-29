class OptionParser
  class ParseError < StandardError
    attr_reader :args
    def initialize(*args)
      @args = args
      # Build the full "reason: arg arg" message at construction so the raise-time
      # message (the one shown for an uncaught error) already reads like MRI's,
      # rather than only #message reconstructing it after the fact.
      super(reason + ": " + args.join(' '))
    end
    def self.reason; "parse error"; end
    def reason; self.class.reason; end
    def message
      reason + ": " + args.join(' ')
    end
    alias to_s message
    def recover(argv)
      argv[0, 0] = @args
      argv
    end
  end

  class InvalidOption < ParseError
    def self.reason; "invalid option"; end
  end
  class MissingArgument < ParseError
    def self.reason; "missing argument"; end
  end
  class InvalidArgument < ParseError
    def self.reason; "invalid argument"; end
  end
  class AmbiguousOption < ParseError
    def self.reason; "ambiguous option"; end
  end
  class AmbiguousArgument < InvalidArgument
    def self.reason; "ambiguous argument"; end
  end
  class NeedlessArgument < ParseError
    def self.reason; "needless argument"; end
  end

  Switch = Struct.new(:short, :long, :arg, :mandatory, :optional, :negatable,
                      :conv, :desc, :block)

  def initialize(banner = nil, width = 32, indent = "    ")
    @banner = banner
    @summary_width = width
    @summary_indent = indent
    @program_name = nil
    @version = nil
    @release = nil
    @default_argv = []
    @switches_long = {}
    @switches_short = {}
    @list = []
    @acceptables = {}
    accept_defaults
    yield self if block_given?
  end

  attr_accessor :banner, :summary_width, :summary_indent, :default_argv
  attr_writer :program_name, :version, :release

  def program_name
    @program_name || File.basename($0 || "optparse")
  end

  def version; @version; end
  def release; @release; end

  def ver
    return nil unless @version
    str = "#{program_name} #{@version}"
    str += " (#{@release})" if @release
    str
  end

  # --- acceptables ---------------------------------------------------------
  def accept_defaults
    accept(Object) { |s| s }
    accept(NilClass) { |s| s }
    accept(String) { |s| s }
    accept(Integer) do |s|
      raise InvalidArgument, s unless s
      if s =~ /\A[-+]?0[xX][0-9a-fA-F_]+\z/
        Integer(s.gsub("_", ""), 16)
      elsif s =~ /\A[-+]?0[bB][01_]+\z/
        Integer(s.gsub("_", ""), 2)
      elsif s =~ /\A[-+]?0[oO][0-7_]+\z/
        Integer(s.gsub("_", ""), 8)
      elsif s =~ /\A[-+]?\d[\d_]*\z/
        Integer(s.gsub("_", ""))
      else
        raise InvalidArgument, s
      end
    end
    accept(Float) do |s|
      unless s && s =~ /\A[-+]?(\d+\.?\d*|\.\d+)([eE][-+]?\d+)?\z/
        raise InvalidArgument, s
      end
      Float(s)
    end
    accept(Array) do |s|
      s ? s.split(',') : s
    end
  end

  def accept(t, &block)
    @acceptables[t] = block
    self
  end

  def reject(t)
    @acceptables.delete(t)
    self
  end

  # --- declaration ---------------------------------------------------------
  def on(*opts, &block)
    register(make_switch(opts, block), :tail)
    self
  end

  def on_head(*opts, &block)
    register(make_switch(opts, block), :head)
    self
  end

  def on_tail(*opts, &block)
    register(make_switch(opts, block), :tail)
    self
  end

  def define(*opts, &block)
    on(*opts, &block)
  end

  def def_option(*opts, &block)
    on(*opts, &block)
  end

  def separator(text = "")
    @list << [:sep, text]
    self
  end

  def register(sw, where)
    item = [:switch, sw]
    if where == :head
      @list.unshift(item)
    else
      @list << item
    end
    sw.short.each { |s| @switches_short[s] = sw }
    sw.long.each { |l| @switches_long[l] = sw }
    if sw.negatable
      sw.long.each do |l|
        @switches_long[l.sub(/\A--/, "--no-")] = sw
      end
    end
    sw
  end

  def make_switch(opts, block)
    short = []
    long = []
    arg = nil
    mandatory = false
    optional = false
    negatable = false
    conv = nil
    desc = []

    opts.each do |o|
      case o
      when Class, Module
        conv = @acceptables[o]
      when Array, Hash
        conv = list_converter(o)
      when /\A--\[no-\]([^\s=]+)(.*)?\z/
        long << "--#{$1}"
        negatable = true
        if $2 && !$2.empty?
          mandatory, optional, arg = parse_argspec($2)
        end
      when /\A--([^\s=\[]+)(.*)?\z/
        long << "--#{$1}"
        if $2 && !$2.empty?
          mandatory, optional, arg = parse_argspec($2)
        end
      when /\A-(.)(.*)?\z/
        short << "-#{$1}"
        if $2 && !$2.empty?
          mandatory, optional, arg = parse_argspec($2)
        end
      when String
        desc << o
      end
    end

    Switch.new(short, long, arg, mandatory, optional, negatable, conv, desc, block)
  end

  def list_converter(o)
    map = {}
    if o.is_a?(Hash)
      o.each { |k, v| map[k.to_s] = v }
    else
      o.each { |v| map[v.to_s] = v }
    end
    lambda do |s|
      exact = map.keys.select { |k| k == s }
      cands = exact.empty? ? map.keys.select { |k| k.start_with?(s) } : exact
      if cands.size == 1
        map[cands[0]]
      elsif cands.empty?
        raise InvalidArgument, s
      else
        raise AmbiguousArgument, s
      end
    end
  end

  def parse_argspec(rest)
    rest = rest.sub(/\A[=\s]+/, "")
    if rest =~ /\A\[(.*)\]\z/
      [false, true, $1]
    elsif !rest.empty?
      [true, false, rest]
    else
      [false, false, nil]
    end
  end

  # --- parsing -------------------------------------------------------------
  def parse(argv = default_argv)
    parse!(argv.dup)
  end

  def parse!(argv = default_argv)
    permute!(argv)
  end

  def order(argv = default_argv, &block)
    order!(argv.dup, &block)
  end

  def order!(argv = default_argv, &nonopt)
    parse_in_order(argv, false, &nonopt)
  end

  def permute(argv = default_argv)
    permute!(argv.dup)
  end

  def permute!(argv = default_argv)
    parse_in_order(argv, true)
  end

  def parse_in_order(argv, permute, &nonopt)
    operands = []
    until argv.empty?
      arg = argv[0]
      if arg == "--"
        argv.shift
        operands.concat(argv)
        argv.clear
        break
      elsif arg == "-" || arg !~ /\A-./
        if permute
          operands << argv.shift
        elsif nonopt
          nonopt.call(argv.shift)
        else
          break
        end
      elsif arg =~ /\A--/
        argv.shift
        parse_long(arg, argv)
      else
        argv.shift
        parse_short(arg, argv)
      end
    end
    argv.concat(operands) if permute
    argv
  end

  def parse_long(arg, argv)
    body = arg[2..]
    name, eq, val = body.partition("=")
    has_eq = !eq.empty?
    val = has_eq ? val : nil
    opt = "--#{name}"
    found = lookup_long(opt)
    raise InvalidOption, arg if found.nil?
    sw, negated = found
    consume(sw, opt, arg, val, has_eq, argv, negated)
  end

  def lookup_long(opt)
    if @switches_long.key?(opt)
      sw = @switches_long[opt]
      return [sw, negated_name?(opt, sw)]
    end
    cands = @switches_long.keys.select { |k| k.start_with?(opt) }
    return nil if cands.empty?
    sws = cands.map { |k| @switches_long[k] }.uniq
    if sws.size == 1
      key = cands.size == 1 ? cands[0] : opt
      return [sws[0], negated_name?(key, sws[0])]
    end
    raise AmbiguousOption, opt
  end

  def negated_name?(key, sw)
    sw.negatable && key.start_with?("--no-") && !sw.long.include?(key)
  end

  def parse_short(arg, argv)
    chars = arg[1..]
    i = 0
    while i < chars.length
      c = "-#{chars[i]}"
      sw = @switches_short[c]
      sw = complete_short(chars[i]) if sw.nil?
      raise InvalidOption, c if sw.nil?
      rest = chars[(i + 1)..]
      if sw.mandatory || sw.optional
        val = nil
        if rest && !rest.empty?
          val = rest.start_with?("=") ? rest[1..] : rest
        end
        consume(sw, c, arg, val, false, argv, false)
        return
      else
        consume(sw, c, arg, nil, false, argv, false)
        i += 1
      end
    end
  end

  # complete_short resolves an unregistered short character against the long
  # options by first letter, the way MRI lets "-n" stand in for "--name" when no
  # "-n" switch exists and exactly one long option begins with "n". A tie raises
  # AmbiguousOption; no match returns nil (the caller raises InvalidOption).
  def complete_short(ch)
    sws = []
    @switches_long.each do |name, sw|
      next if name.start_with?("--no-") && !sw.long.include?(name)
      sws << sw if name[2] == ch
    end
    sws.uniq!
    raise AmbiguousOption, "-#{ch}" if sws.size > 1
    sws[0]
  end

  def consume(sw, opt, orig, val, has_eq, argv, negated)
    if sw.mandatory
      if val.nil?
        raise MissingArgument, opt if argv.empty?
        val = argv.shift
      end
      sw.block.call(convert(sw, val, orig)) if sw.block
    elsif sw.optional
      if val.nil? && !argv.empty? && !(argv[0] =~ /\A-./)
        val = argv.shift
      end
      cval = val.nil? ? nil : convert(sw, val, orig)
      sw.block.call(cval) if sw.block
    else
      raise NeedlessArgument, orig if has_eq && !val.nil?
      sw.block.call(sw.negatable ? !negated : true) if sw.block
    end
  end

  def convert(sw, val, orig)
    return val unless sw.conv
    begin
      sw.conv.call(val)
    rescue InvalidArgument
      raise InvalidArgument.new(*split_invalid(orig, val))
    rescue ArgumentError, TypeError
      raise InvalidArgument.new(*split_invalid(orig, val))
    end
  end

  def split_invalid(orig, val)
    if orig =~ /\A(--[^\s=]+)\z/
      [$1, val]
    else
      [orig]
    end
  end

  # --- getopts -------------------------------------------------------------
  def getopts(*args)
    argv = args.first.is_a?(Array) ? args.shift : default_argv
    result = {}
    args.each do |spec|
      spec.scan(/([a-zA-Z0-9][a-zA-Z0-9_-]*)(:)?/) do |name, takes_arg|
        flag = name.length == 1 ? "-#{name}" : "--#{name}"
        if takes_arg
          result[name] = nil
          on("#{flag} VALUE") { |v| result[name] = v }
        else
          result[name] = false
          on(flag) { result[name] = true }
        end
      end
    end
    parse!(argv)
    result
  end

  # --- help ----------------------------------------------------------------
  def help
    to_s
  end

  def to_s
    out = +""
    out << (banner || "Usage: #{program_name} [options]") << "\n"
    @list.each do |kind, obj|
      if kind == :sep
        out << obj << "\n"
      else
        summarize_switch(obj).each { |l| out << l }
      end
    end
    out
  end

  def summarize(to = [])
    @list.each do |kind, obj|
      if kind == :sep
        to << obj + "\n"
      else
        summarize_switch(obj).each { |l| to << l }
      end
    end
    to
  end

  def summarize_switch(sw)
    indent = @summary_indent
    width = @summary_width
    left = build_left(sw)
    lines = []
    desc = sw.desc.dup
    if left.length <= width
      first = desc.shift
      padded = left + (" " * (width - left.length))
      line = indent + padded
      line = (line + " " + first).rstrip if first
      lines << line + "\n"
    else
      lines << indent + left + "\n"
    end
    desc.each do |d|
      lines << indent + (" " * width) + " " + d + "\n"
    end
    lines
  end

  def build_left(sw)
    argstr = if sw.mandatory
      sw.arg ? " #{sw.arg}" : ""
    elsif sw.optional
      sw.arg ? " [#{sw.arg}]" : ""
    else
      ""
    end
    long = sw.long.dup
    long = long.map { |l| l.sub(/\A--/, "--[no-]") } if sw.negatable
    short_str = sw.short.join(", ")
    long_str = long.join(", ")
    if !short_str.empty? && !long_str.empty?
      "#{short_str}, #{long_str}#{argstr}"
    elsif !short_str.empty?
      "#{short_str}#{argstr}"
    else
      "    #{long_str}#{argstr}"
    end
  end
end

OptParse = OptionParser
