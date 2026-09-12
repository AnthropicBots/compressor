#include "../include/Decompressor.h"
#include <fstream>
#include <iostream>
#include <bitset>

void Decompressor::decompress(const string& inputFile, const string& outputFile) {

    ifstream in(inputFile, ios::binary);
    if (!in) {
        cout << "Error: Cannot open compressed file\n";
        return;
    }

    // -------- READ HEADER --------
    int originalSize;
    in.read((char*)&originalSize, sizeof(originalSize));

    int tableSize;
    in.read((char*)&tableSize, sizeof(tableSize));

    map<char, int> freq;

    for (int i = 0; i < tableSize; i++) {
        char ch;
        int f;
        in.get(ch);
        in.read((char*)&f, sizeof(f));
        freq[ch] = f;
    }

    // -------- REBUILD EXACT TREE --------
    HuffmanTree tree;
    tree.setFrequency(freq);
    tree.buildTree();

    HuffmanNode* root = tree.getRoot();

    // -------- READ BINARY DATA --------
    string bits = "";
    char byte;

    while (in.get(byte))
        bits += bitset<8>((unsigned char)byte).to_string();

    in.close();

    // -------- DECODE --------
    ofstream out(outputFile, ios::binary);
    HuffmanNode* current = root;
    int written = 0;

    for (char bit : bits) {

        if (bit == '0')
            current = current->left;
        else
            current = current->right;

        if (!current->left && !current->right) {
            out.put(current->data);
            current = root;
            written++;

            if (written == originalSize)
                break;
        }
    }

    out.close();
    cout << "Decompression complete!\n";
}
