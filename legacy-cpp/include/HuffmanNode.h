#ifndef HUFFMANNODE_H
#define HUFFMANNODE_H

class HuffmanNode {
public:
    char data;
    int freq;
    HuffmanNode* left;
    HuffmanNode* right;

    HuffmanNode(char d, int f) {
        data = d;
        freq = f;
        left = right = nullptr;
    }
};

#endif
