#include <iostream>
#include "include/Compressor.h"
#include "include/Decompressor.h"

using namespace std;

int main() {

    int choice;
    Compressor comp;
    Decompressor decomp;

    cout << "==== Huffman File Compressor ====\n";
    cout << "1. Compress File\n";
    cout << "2. Decompress File\n";
    cout << "3. Exit\n";
    cout << "Enter choice: ";
    cin >> choice;

    if (choice == 1)
        comp.compress("input/test.txt", "output/compressed.bin");

    else if (choice == 2)
        decomp.decompress("output/compressed.bin", "output/decompressed.txt");

    else
        cout << "Program Ended\n";

    return 0;
}
