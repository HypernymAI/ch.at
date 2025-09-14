/**
 * Custom DNS resolver that supports non-standard ports
 */

import * as dgram from 'dgram';
import * as dnsPacket from 'dns-packet';

export class CustomResolver {
  private host: string;
  private port: number;
  
  constructor(server: string) {
    const parts = server.split(':');
    this.host = parts[0];
    this.port = parts[1] ? parseInt(parts[1]) : 53;
  }
  
  getServers(): string[] {
    return [`${this.host}:${this.port}`];
  }
  
  resolveTxt(domain: string): Promise<string[][]> {
    return new Promise((resolve, reject) => {
      console.log(`[CustomResolver] Resolving: ${domain}`);
      const socket = dgram.createSocket('udp4');
      const id = Math.floor(Math.random() * 65535);
      
      const query = dnsPacket.encode({
        id,
        type: 'query',
        flags: dnsPacket.RECURSION_DESIRED,
        questions: [{
          type: 'TXT',
          name: domain
        }]
      });
      
      const timeout = setTimeout(() => {
        console.log(`[CustomResolver] Timeout for ${domain}`);
        socket.close();
        reject(new Error(`queryTxt ENOTFOUND ${domain}`));
      }, 5000);
      
      socket.on('message', (msg) => {
        console.log(`[CustomResolver] Received response for ${domain}, size: ${msg.length}`);
        clearTimeout(timeout);
        socket.close();
        
        try {
          const response = dnsPacket.decode(msg);
          
          if (response.id !== id) {
            reject(new Error('DNS response ID mismatch'));
            return;
          }
          
          const txtRecords = (response.answers || [])
            .filter(answer => answer.type === 'TXT')
            .map(answer => {
              // dns-packet returns TXT data as Buffer, convert to string array
              const data = answer.data;
              if (Buffer.isBuffer(data)) {
                return [data.toString()];
              }
              return data as string[];
            });
          
            
          resolve(txtRecords);
        } catch (err) {
          reject(err);
        }
      });
      
      socket.on('error', (err) => {
        clearTimeout(timeout);
        socket.close();
        reject(err);
      });
      
      socket.send(query, this.port, this.host, (err) => {
        if (err) {
          console.error('[CustomResolver] Send error:', err);
          clearTimeout(timeout);
          socket.close();
          reject(err);
        } else {
          console.log(`[CustomResolver] Sent query for ${domain}`);
        }
      });
    });
  }
}