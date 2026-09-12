#include "../include/HuffmanTree.h"

struct Compare {
    bool operator()(HuffmanNode* left, HuffmanNode* right) {
        return left->freq > right->freq;
    }
};

HuffmanTree::HuffmanTree() {
    root = nullptr;
}
void HuffmanTree::setFrequency(const map<char,int>& f) {
    freq = f;
}


void HuffmanTree::buildFrequency(const string& text) {
    for (char ch : text)
        freq[ch]++;
}

void HuffmanTree::buildTree() {
    priority_queue<HuffmanNode*, vector<HuffmanNode*>, Compare> minHeap;

    for (auto pair : freq)
        minHeap.push(new HuffmanNode(pair.first, pair.second));

    while (minHeap.size() > 1) {
        HuffmanNode* left = minHeap.top(); minHeap.pop();
        HuffmanNode* right = minHeap.top(); minHeap.pop();

        HuffmanNode* parent = new HuffmanNode('$', left->freq + right->freq);
        parent->left = left;
        parent->right = right;

        minHeap.push(parent);
    }

    root = minHeap.top();
}

void HuffmanTree::generateCodes(HuffmanNode* node, string code) {
    if (!node) return;

    if (node->data != '$')
        codes[node->data] = code;

    generateCodes(node->left, code + "0");
    generateCodes(node->right, code + "1");
}

void HuffmanTree::createCodes() {
    generateCodes(root, "");
}

map<char, string> HuffmanTree::getCodes() {
    return codes;
}

map<char, int> HuffmanTree::getFrequency() {
    return freq;
}

HuffmanNode* HuffmanTree::getRoot() {
    return root;
}
