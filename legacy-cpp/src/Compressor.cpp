#include "../include/Compressor.h"
#include <fstream>
#include <iostream>

void Compressor::compress(const string& inputFile, const string& outputFile) {

    // 1️⃣ Read file safely
    ifstream in(inputFile, ios::binary);
    if (!in) {
        cout << "Error: Cannot open input file\n";
        return;
    }

    string text((istreambuf_iterator<char>(in)),
                 istreambuf_iterator<char>());

    in.close();
    cout << "File read successfully\n";

    // 2️⃣ Build Huffman Tree
    HuffmanTree tree;
    tree.buildFrequency(text);
    tree.buildTree();
    tree.createCodes();

    auto codes = tree.getCodes();
    auto freq  = tree.getFrequency();

    // 3️⃣ Convert text → binary string
    string encoded = "";
    for (char ch : text)
        encoded += codes[ch];

    // 4️⃣ Write compressed file (HEADER + DATA)
    ofstream out(outputFile, ios::binary);
    if (!out) {
        cout << "Error: Cannot open output file\n";
        return;
    }

    // -------- WRITE HEADER (frequency table) --------
    // -------- WRITE HEADER --------

// write original text length
int originalSize = text.size();
out.write((char*)&originalSize, sizeof(originalSize));

// write frequency table size
int tableSize = freq.size();
out.write((char*)&tableSize, sizeof(tableSize));


    for (auto pair : freq) {
        out.put(pair.first);                       // character
        out.write((char*)&pair.second, sizeof(pair.second)); // frequency
    }

    // -------- WRITE BINARY DATA --------
    unsigned char byte = 0;
    int bitCount = 0;

    for (char bit : encoded) {
        byte = (byte << 1) | (bit - '0');
        bitCount++;

        if (bitCount == 8) {
            out.put(byte);
            byte = 0;
            bitCount = 0;
        }
    }

    // padding remaining bits
    if (bitCount > 0) {
        byte <<= (8 - bitCount);
        out.put(byte);
    }

    out.close();

    cout << "Compression complete!\n";
}
