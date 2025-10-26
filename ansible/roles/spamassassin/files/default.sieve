require ["fileinto", "mailbox"];

# Move spam to Junk folder
if header :contains "X-Spam-Flag" "YES" {
  fileinto :create "Junk";
  stop;
}

# Move high-score spam directly
if header :contains "X-Spam-Level" "**********" {
  fileinto :create "Junk";
  stop;
}
