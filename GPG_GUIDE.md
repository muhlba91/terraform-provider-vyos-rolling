# GPG and Git Configuration Guide

This guide provides step-by-step instructions for generating GPG keys, configuring Git to use them for signing commits and tags, and exporting your public and private keys.

## 1. Generating a GPG Key

If you don't have a GPG key, you can generate one using the following command:

```bash
gpg --full-generate-key
```

Follow the prompts to create your key. It is recommended to use a strong password for your key.

## 2. Listing Your GPG Keys and Finding the Key ID

To list your GPG keys and find your key ID, use the following command:

```bash
gpg --list-secret-keys --keyid-format LONG
```

The output will look something like this:

```
/root/.gnupg/pubring.kbx
------------------------
sec   rsa4096/F71BA86F1325C242 2025-11-23 [SC]
      244C8B3281AA3A9017FEC0A7F71BA86F1325C242
uid                 [ultimate] xxxxxx <xxxx@xxxx.com>
ssb   rsa4096/8840D77A98BAFB2B 2025-11-23 [E]
```

In this example, the **Key ID** is `F71BA86F1325C242`.

## 3. Configuring Git to Use Your GPG Key

You need to configure Git to use your GPG key to sign commits and tags.

First, configure your user name and email to match the GPG key's user ID:

```bash
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"
```

Next, tell Git which key to use for signing:

```bash
git config --global user.signingkey F71BA86F1325C242
```

Replace `F71BA86F1325C242` with your Key ID.

You can also configure Git to automatically sign all commits:

```bash
git config --global commit.gpgsign true
```

And to sign tags:

```bash
git config --global tag.gpgsign true
```

## 4. Exporting Your Public GPG Key

To export your public GPG key, use the following command:

```bash
gpg --armor --export F71BA86F1325C242
```

Replace `F71BA86F1325C242` with your Key ID. This will print the public key to the console. You can then copy and paste it where needed (e.g., GitHub).

## 5. Exporting and Backing Up Your Private GPG Key

It is very important to back up your private key. If you lose it, you will not be able to sign anything with it anymore.

To export your private key, use the following command:

```bash
gpg --armor --export-secret-keys F71BA86F1325C242
```

Replace `F71BA86F1325C242` with your Key ID.

**Treat your private key with extreme care.** Anyone who has access to your private key can sign commits and tags as you. Store it in a secure location, like a password manager or an encrypted USB drive.
