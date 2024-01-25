#!/usr/bin/env ruby

require 'octokit'
require 'uri'

puts "Starting ghbackup..."

begin
  Octokit.configure do |c|
    c.auto_paginate = true
  end

  github_secret = ENV["GITHUB_SECRET"]
  backup_folder = ENV["BACKUP_FOLDER"] || "/ghbackup"

  client = Octokit::Client.new(access_token: github_secret)

  login = client.user[:login]

  client.repos.each do |repo|
    uri = URI.parse(repo[:clone_url])
    authenitcated_clone_url = "#{uri.scheme}://#{login}:#{github_secret}@#{uri.host}#{uri.path}"
    unauthenticated_clone_url = "#{uri.scheme}://#{uri.host}#{uri.path}"

    backup_path = "#{backup_folder}/#{repo[:full_name]}.git"

    puts "\nBacking up #{repo[:full_name]}..."

    system('git', 'config', '--global', '--add', 'safe.directory', '*')

    if Dir.exist?(backup_path)
      Dir.chdir(backup_path) {
        system('git', 'remote', 'set-url', 'origin', authenitcated_clone_url)
        system('git', 'remote', 'update')
        system('git', 'lfs', 'fetch', '--all')
        system('git', 'remote', 'set-url', 'origin', unauthenticated_clone_url)
      }
    else
      system('git', 'clone', '--mirror', '--no-checkout', '--progress', authenitcated_clone_url, backup_path)
      Dir.chdir(backup_path) {
        system('git', 'lfs', 'fetch', '--all')
        system('git', 'remote', 'set-url', 'origin', unauthenticated_clone_url)
      }
    end
  end

  puts "\nBackup complete!"
rescue => e
  puts "Error: #{e}"
end
