class Gitflow < Formula
  desc "Git Flow workflow helper with interactive TUI and CLI"
  homepage "https://github.com/novaemx/gitflow-helper"
  url "https://github.com/novaemx/gitflow-helper/releases/download/v0.5.52/gitflow-0.5.52-darwin-universal.tar.gz"
  version "0.5.52"
  sha256 "61f92f4e97de2bc757cad2f637e91b00e012be3bc98ea0522500f347863d429f"
  license "MIT"

  def install
    bin.install "gitflow"
  end

  test do
    output = shell_output("#{bin}/gitflow --version")
    assert_match version.to_s, output
  end
end
