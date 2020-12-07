#!/usr/bin/env python3

import re, sys, os, subprocess, hashlib, getopt
import urllib.request

# Get the correct name for the rpms
def fix_name(name):
    if name == "gperftools-lib":
        return "gperftools-libs"
    if name == "ncurses-lib":
        return "ncurses-libs"
    return name

# Get the sha256sum
def get_sha256(url):
    hash = hashlib.sha256()
    remote = urllib.request.urlopen(url)
    hash.update(remote.read())
    return hash.hexdigest()

# Write the file o with the http_file and updated fields
def update_http_output(o, name, url, sha):
    o.writelines([
    'http_file(\n', 
    '    name = "'+name+'",\n',
    '    sha256 = "'+sha+'",\n',
    '    urls = [\n',
    '        "'+url+'",\n',
    '   ],\n',
    ')\n\n'])

def get_name_from_line(line):
    return re.search('\".*?\"', line).group().strip('"')

# Get the url for the dependency using yumdownloader
def get_url(name):
    fixname = fix_name(name)
    print("Fetch dep:", name)
    result = subprocess.run(["/usr/bin/yumdownloader", "--url", "--urlprotocols", "http", fixname] , stdout=subprocess.PIPE)
    # Avoid the rpms for i686
    return re.search('http\:.[^\n]*[^i686]\.rpm', result.stdout.decode("utf-8") , re.DOTALL).group().strip('"')


inputfile = ''
outputfile = ''

opts, args = getopt.getopt(sys.argv[1:],"hi:o:w",["ifile=","ofile="])
for opt, arg in opts:
   if opt == '-h':
      print("test.py: \n  -i <inputfile>\n  -w overwrite inputfile\n  -o <outputfile>\n  (-o and -w are mutually exclusive)")
      sys.exit()
   elif opt in ("-i", "--ifile"):
      inputfile = arg
   elif opt in ("-w"):
       outputfile = inputfile
   elif opt in ("-o", "--ofile") and opt != "-w":
      outputfile = arg

if inputfile == "":
    print('Provide input file: -i <inputfile>')
    sys.exit()

if outputfile == "":
    print('Provide output file: -o <outputfile> or -w to overwrite the inputfile')
    sys.exit()

# Read the input file
f = open(inputfile, "r")
lines = f.readlines()
f.close()

# Remove output file if exists
if os.path.exists(outputfile):
  os.remove(outputfile)
o = open(outputfile, 'a+')

find_next_parenthesis = False
for l in lines:
    # Get the name of the package
    if find_next_parenthesis and "name =" in l:
        name = get_name_from_line(l)
        url = get_url(name)
        sha = get_sha256(url)
        update_http_output(o, name, url, sha) 

    # Loop until we find the next ')'
    if "http_file(" in l : 
        find_next_parenthesis = True
        continue

    if find_next_parenthesis and l == ")":
        find_next_parenthesis = False
        continue
    if find_next_parenthesis:
        continue

    o.write(l)
