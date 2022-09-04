#!/usr/bin/env ruby

require 'octokit'
require 'uri'

interval = ENV["INTERVAL"].to_i || 3600

puts "Starting ghbackup..."
puts "Running as user #{Process.uid} and group #{Process.gid}."

sleep interval

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
    
    backup_path = "#{backup_folder}/#{repo[:full_name]}.git"

    p "Backing up #{repo[:full_name]}..."

    if Dir.exist?(backup_path)
      Dir.chdir(backup_path) {
        system('git', 'remote', 'update')
        system('git', 'lfs', 'fetch', '--all')
      }
    else
      system('git', 'clone', '--mirror', '--no-checkout', '--progress', authenitcated_clone_url, backup_path)
      Dir.chdir(backup_path) {
        system('git', 'lfs', 'fetch', '--all')
      }
    end
  end
rescue => e
  puts "Error: #{e}"
end
