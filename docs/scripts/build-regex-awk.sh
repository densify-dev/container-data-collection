#!/bin/bash
#
# Generates a highly compact and correct regular expression from a list of strings.
# This version fixes a critical bug that allowed partial matches.
#
# Usage: ./build-regex-awk.sh <input_file>
#

set -o pipefail

if [[ $# -ne 1 || ! -r "$1" ]]; then
  echo "Usage: $0 <input_file>" >&2
  exit 1
fi

# This awk script builds a prefix tree and then traverses it to generate the regex.
# It is designed to be robust and correct, avoiding partial matches.
awk '
  # Part 1: Build the prefix tree
  {
    if (NF > 0) {
      n = split($0, parts, "_");
      current_path = "";
      for (i = 1; i <= n; i++) {
        parent_path = current_path;
        current_path = current_path (current_path ? "/" : "") parts[i];
        children[parent_path][parts[i]] = 1;
      }
      # Mark the end of a valid metric name
      is_leaf[current_path] = 1;
    }
  }

  # Part 2: After reading all lines, generate the regex from the tree
  END {
    regex = generate_group("");
    print "^(" regex ")$";
  }

  # Function to generate regex for a group of children
  function generate_group(parent_path,   # local variables
                          child_name, sub_regex, group_parts, num_children, i, sorted_children,
                          child_path, has_sub_group, is_hybrid, hybrid_parts) {

    if (!(parent_path in children)) return "";

    # Sort children for deterministic output
    num_children = 0;
    for (child_name in children[parent_path]) {
      sorted_children[++num_children] = child_name;
    }
    asort(sorted_children);

    group_parts = "";
    for (i = 1; i <= num_children; i++) {
      child_name = sorted_children[i];
      child_path = parent_path (parent_path ? "/" : "") child_name;
      
      # Escape the child name for regex safety
      gsub(/[.^$*+?()|[\]{}\\]/, "\\\\&", child_name);

      sub_regex = generate_group(child_path);
      has_sub_group = (sub_regex != "");
      is_hybrid = is_leaf[child_path] && has_sub_group;

      # === CORRECTED LOGIC ===
      if (is_hybrid) {
        # Node is both an endpoint and has children.
        # Create a non-capturing group for the suffix, making it optional.
        # e.g., for "a" and "a_b", generates "a(?:_b)?"
        hybrid_parts = child_name "(?:_" sub_regex ")?";
        group_parts = group_parts (group_parts ? "|" : "") hybrid_parts;
      } else if (has_sub_group) {
        # Node only has children, not an endpoint itself.
        # e.g., for "a_b", generates "a_b"
        group_parts = group_parts (group_parts ? "|" : "") child_name "_" sub_regex;
      } else {
        # Node is a leaf (endpoint) with no children.
        group_parts = group_parts (group_parts ? "|" : "") child_name;
      }
    }

    # If there is more than one branch, wrap in parentheses
    if (num_children > 1) {
      return "(" group_parts ")";
    }
    return group_parts;
  }
' "$1"
