import time

import torch
import torch.nn as nn
import torch.nn.functional as F
import torch.optim as optim

# Training settings
batch_size = 64


class Net(nn.Module):
    def __init__(self):
        super(Net, self).__init__()
        self.fc = nn.Linear(784, 10)

    def forward(self, x):
        return self.fc(x.view(x.size(0), -1))


device = 'cuda' if torch.cuda.is_available() else 'cpu'
model = Net().to(device)
optimizer = optim.SGD(model.parameters(), lr=0.01, momentum=0.5)


def train(epoch):
    loss100 = 0.0
    start = time.time()
    for i in range(100000000):
        inputs = torch.randn(64, 1, 28, 28)
        target = torch.randint(0, 9, (64,)).long()

        inputs = inputs.to(device)
        target = target.to(device)
        optimizer.zero_grad()
        output = model(inputs)
        loss = F.cross_entropy(output, target)
        optimizer.zero_grad()
        loss.backward()
        optimizer.step()
        loss100 += loss.item()
        if i % 1000 == 0:
            end = time.time()
            print('[Epoch %d, Batch %5d] loss: %.3f time: %s' %
                  (epoch + 1, i + 1, loss100 / 100, end - start))
            loss100 = 0.0
            start = time.time()


train(0)